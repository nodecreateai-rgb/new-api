package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	pigo "github.com/esimov/pigo/core"
	"golang.org/x/image/webp"
)

//go:embed cascade/facefinder
var complianceFaceCascadeBytes []byte

//go:embed cascade/puploc
var compliancePupilCascadeBytes []byte

const (
	complianceMaxImageBytes     = 32 << 20
	complianceDetectMaxEdge     = 1024
	complianceSmallFaceMinPx    = 34
	complianceSmallFaceMinScore = 18.0
)

var (
	complianceDetectorsOnce sync.Once
	complianceFaceCascade   *pigo.Pigo
	compliancePupilCascade  *pigo.PuplocCascade
	complianceDetectorsErr  error
)

type complianceFace struct {
	CenterX    float64
	CenterY    float64
	Size       float64
	LeftEyeX   float64
	LeftEyeY   float64
	RightEyeX  float64
	RightEyeY  float64
	Confidence float32
}

func loadComplianceDetectors() (*pigo.Pigo, *pigo.PuplocCascade, error) {
	complianceDetectorsOnce.Do(func() {
		face, err := pigo.NewPigo().Unpack(complianceFaceCascadeBytes)
		if err != nil {
			complianceDetectorsErr = fmt.Errorf("unpack facefinder cascade: %w", err)
			return
		}
		pupil, err := pigo.NewPuplocCascade().UnpackCascade(compliancePupilCascadeBytes)
		if err != nil {
			complianceDetectorsErr = fmt.Errorf("unpack puploc cascade: %w", err)
			return
		}
		complianceFaceCascade = face
		compliancePupilCascade = pupil
	})
	if complianceDetectorsErr != nil {
		return nil, nil, complianceDetectorsErr
	}
	return complianceFaceCascade, compliancePupilCascade, nil
}

// editComplianceImageLocal fetches src, detects human faces locally using pure-Go
// cascades, and applies the selected compliance treatment in place. When no face
// is found, it returns src unchanged with modified=false.
func editComplianceImageLocal(ctx context.Context, mode, src string) (string, bool, error) {
	raw, _, err := fetchComplianceImageBytes(ctx, src)
	if err != nil {
		return "", false, err
	}
	nrgba, format, hasAlpha, err := decodeComplianceNRGBA(raw)
	if err != nil {
		return "", false, err
	}
	w, h := nrgba.Bounds().Dx(), nrgba.Bounds().Dy()
	if w < 24 || h < 24 {
		return src, false, nil
	}
	faces, err := detectComplianceFaces(nrgba)
	if err != nil {
		return "", false, err
	}
	if len(faces) == 0 {
		return src, false, nil
	}

	renderComplianceFaces(nrgba, faces, mode)
	encoded, outMime, err := encodeComplianceNRGBA(nrgba, format, hasAlpha)
	if err != nil {
		return "", false, err
	}
	return "data:" + outMime + ";base64," + base64.StdEncoding.EncodeToString(encoded), true, nil
}

func fetchComplianceImageBytes(ctx context.Context, src string) ([]byte, string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, "", fmt.Errorf("empty compliance image source")
	}
	if strings.HasPrefix(src, "data:") {
		return decodeDataURL(src)
	}
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return nil, "", fmt.Errorf("unsupported compliance image scheme")
	}
	if system_setting.EnableWorker() {
		resp, err := DoDownloadRequest(src, "compliance")
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()
		return readComplianceHTTPResponse(resp)
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(
		src,
		fetchSetting.EnableSSRFProtection,
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	); err != nil {
		return nil, "", fmt.Errorf("compliance image url rejected: %w", err)
	}

	client := complianceDownloadClient()
	var lastErr error
	for attempt := 1; attempt <= downloadRequestMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, src, nil)
		if err != nil {
			cancel()
			return nil, "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			if !isRetryableDownloadError(err) || attempt == downloadRequestMaxAttempts {
				return nil, "", err
			}
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
			continue
		}
		data, ct, readErr := readComplianceHTTPResponse(resp)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, "", readErr
		}
		return data, ct, nil
	}
	return nil, "", lastErr
}

func complianceDownloadClient() *http.Client {
	if c := GetHttpClient(); c != nil {
		return c
	}
	return &http.Client{
		Timeout:       60 * time.Second,
		CheckRedirect: checkRedirect,
	}
}

func readComplianceHTTPResponse(resp *http.Response) ([]byte, string, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download compliance image: http %d", resp.StatusCode)
	}
	if resp.ContentLength > complianceMaxImageBytes {
		return nil, "", fmt.Errorf("compliance image exceeds %d bytes", complianceMaxImageBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, complianceMaxImageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read compliance image: %w", err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("compliance image is empty")
	}
	if len(data) > complianceMaxImageBytes {
		return nil, "", fmt.Errorf("compliance image exceeds %d bytes", complianceMaxImageBytes)
	}
	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx > 0 {
		ct = ct[:idx]
	}
	return data, strings.TrimSpace(ct), nil
}

func decodeComplianceNRGBA(raw []byte) (*image.NRGBA, string, bool, error) {
	orientation := readJPEGExifOrientation(raw)
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		if wimg, werr := webp.Decode(bytes.NewReader(raw)); werr == nil {
			img = wimg
			format = "webp"
			err = nil
		}
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("decode compliance image: %w", err)
	}
	nrgba, hasAlpha := imageToNRGBA(img)
	if orientation > 1 && orientation <= 8 {
		nrgba = applyExifOrientationNRGBA(nrgba, orientation)
	}
	return nrgba, format, hasAlpha, nil
}

func encodeComplianceNRGBA(img *image.NRGBA, format string, hasAlpha bool) ([]byte, string, error) {
	var buf bytes.Buffer
	pixels := img.Bounds().Dx() * img.Bounds().Dy()
	// Prefer lossless PNG up to 2 MP so fine grid seams / fishnet strands stay sharp
	// and pixels outside detected faces remain bit-exact; fall back to high-quality JPEG
	// only on very large opaque frames to keep base64 data URL payloads bounded.
	usePNG := hasAlpha || pixels <= 2_000_000 || format == "png"
	if usePNG {
		if err := commonEncodePNG(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 94}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

func imageToNRGBA(src image.Image) (*image.NRGBA, bool) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	hasAlpha := false
	for i := 3; i < len(dst.Pix); i += 4 {
		if dst.Pix[i] < 255 {
			hasAlpha = true
			break
		}
	}
	return dst, hasAlpha
}

func applyExifOrientationNRGBA(src *image.NRGBA, orientation int) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dstW, dstH := w, h
	if orientation >= 5 && orientation <= 8 {
		dstW, dstH = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			var dx, dy int
			switch orientation {
			case 2: // flip horizontal
				dx, dy = w-1-sx, sy
			case 3: // rotate 180
				dx, dy = w-1-sx, h-1-sy
			case 4: // flip vertical
				dx, dy = sx, h-1-sy
			case 5: // transpose
				dx, dy = sy, sx
			case 6: // rotate 90 CW
				dx, dy = h-1-sy, sx
			case 7: // transverse
				dx, dy = h-1-sy, w-1-sx
			case 8: // rotate 270 CW
				dx, dy = sy, w-1-sx
			default:
				dx, dy = sx, sy
			}
			si := sy*src.Stride + sx*4
			di := dy*dst.Stride + dx*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

// readJPEGExifOrientation extracts EXIF orientation tag (1..8) from raw JPEG bytes.
func readJPEGExifOrientation(raw []byte) int {
	if len(raw) < 12 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return 1
	}
	pos := 2
	for pos+4 <= len(raw) {
		if raw[pos] != 0xFF {
			break
		}
		marker := raw[pos+1]
		pos += 2
		if marker == 0xD9 || marker == 0xDA {
			break
		}
		if marker == 0x00 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			continue
		}
		if pos+2 > len(raw) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(raw[pos : pos+2]))
		if segLen < 2 || pos+segLen > len(raw) {
			break
		}
		seg := raw[pos+2 : pos+segLen]
		pos += segLen
		if marker != 0xE1 || len(seg) < 14 || !bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			continue
		}
		tiff := seg[6:]
		var order binary.ByteOrder
		switch {
		case len(tiff) >= 8 && tiff[0] == 'I' && tiff[1] == 'I':
			order = binary.LittleEndian
		case len(tiff) >= 8 && tiff[0] == 'M' && tiff[1] == 'M':
			order = binary.BigEndian
		default:
			return 1
		}
		if order.Uint16(tiff[2:4]) != 0x002A {
			return 1
		}
		ifdOff := int(order.Uint32(tiff[4:8]))
		if ifdOff < 8 || ifdOff+2 > len(tiff) {
			return 1
		}
		entries := int(order.Uint16(tiff[ifdOff : ifdOff+2]))
		for i := 0; i < entries; i++ {
			off := ifdOff + 2 + i*12
			if off+12 > len(tiff) {
				break
			}
			if order.Uint16(tiff[off:off+2]) == 0x0112 {
				val := int(order.Uint16(tiff[off+8 : off+10]))
				if val >= 1 && val <= 8 {
					return val
				}
				return 1
			}
		}
		return 1
	}
	return 1
}

type detectPass struct {
	angle  float64
	mirror bool
	shift  float64
	scale  float64
	minQ   float32
}

var complianceDetectPasses = []detectPass{
	{angle: 0.0, mirror: false, shift: 0.08, scale: 1.08, minQ: 14.0},
	{angle: 0.04, mirror: false, shift: 0.08, scale: 1.08, minQ: 16.0},
	{angle: 0.96, mirror: false, shift: 0.08, scale: 1.08, minQ: 16.0},
	{angle: 0.0, mirror: true, shift: 0.08, scale: 1.08, minQ: 16.0},
}

func detectComplianceFaces(src *image.NRGBA) ([]complianceFace, error) {
	faceClassifier, pupilClassifier, err := loadComplianceDetectors()
	if err != nil {
		return nil, err
	}
	gray, detW, detH, scaleRatio := buildDetectionGray(src, complianceDetectMaxEdge)

	// Embed the rectangular image into a slightly padded square canvas so
	// near-edge portraits have margin and pigo's rotated classifier never mixes
	// nrows vs ncols bounds on tall vertical frames.
	maxDim := detW
	if detH > maxDim {
		maxDim = detH
	}
	side := int(math.Ceil(float64(maxDim) * 1.15))
	padX := (side - detW) / 2
	padY := (side - detH) / 2
	squareGray := makeSquareGray(gray, detW, detH, side, padX, padY)
	squareMirrored := mirrorSquareHoriz(squareGray, side)

	minSide := detW
	if detH < minSide {
		minSide = detH
	}
	minSize := max(20, minSide/18)
	maxSize := min(side-2, int(float64(minSide)*1.25))
	if maxSize < minSize {
		return nil, nil
	}

	var candidates []pigo.Detection
	for _, pass := range complianceDetectPasses {
		pix := squareGray
		if pass.mirror {
			pix = squareMirrored
		}
		params := pigo.CascadeParams{
			MinSize:     minSize,
			MaxSize:     maxSize,
			ShiftFactor: pass.shift,
			ScaleFactor: pass.scale,
			ImageParams: pigo.ImageParams{
				Pixels: pix,
				Rows:   side,
				Cols:   side,
				Dim:    side,
			},
		}
		clustered := faceClassifier.ClusterDetections(faceClassifier.RunCascade(params, pass.angle), 0.2)
		for _, det := range clustered {
			if det.Scale < complianceSmallFaceMinPx && det.Q < complianceSmallFaceMinScore {
				continue
			}
			if det.Q < pass.minQ {
				continue
			}
			col := det.Col - padX
			row := det.Row - padY
			if pass.mirror {
				col = (side - 1 - det.Col) - padX
			}
			// Discard spurious detections whose centers fell completely outside the image frame.
			if col < 0 || col >= detW || row < 0 || row >= detH {
				continue
			}
			candidates = append(candidates, pigo.Detection{
				Row:   row,
				Col:   col,
				Scale: det.Scale,
				Q:     det.Q,
			})
		}
	}

	merged := suppressOverlappingDetections(candidates, 0.25)
	if len(merged) == 0 {
		return nil, nil
	}

	invScale := 1.0 / scaleRatio
	faces := make([]complianceFace, 0, len(merged))
	for _, det := range merged {
		lx, ly, rx, ry := locateComplianceEyes(pupilClassifier, gray, detW, detH, float64(det.Col), float64(det.Row), float64(det.Scale))
		faces = append(faces, complianceFace{
			CenterX:    float64(det.Col) * invScale,
			CenterY:    float64(det.Row) * invScale,
			Size:       float64(det.Scale) * invScale,
			LeftEyeX:   lx * invScale,
			LeftEyeY:   ly * invScale,
			RightEyeX:  rx * invScale,
			RightEyeY:  ry * invScale,
			Confidence: det.Q,
		})
	}
	sort.Slice(faces, func(i, j int) bool {
		if math.Abs(faces[i].CenterY-faces[j].CenterY) > 20 {
			return faces[i].CenterY < faces[j].CenterY
		}
		return faces[i].CenterX < faces[j].CenterX
	})
	return faces, nil
}

func locateComplianceEyes(plc *pigo.PuplocCascade, gray []uint8, w, h int, cx, cy, size float64) (lx, ly, rx, ry float64) {
	lx = cx - 0.18*size
	ly = cy - 0.08*size
	rx = cx + 0.18*size
	ry = cy - 0.08*size
	if plc == nil || size < 24 {
		return lx, ly, rx, ry
	}
	img := pigo.ImageParams{Pixels: gray, Rows: h, Cols: w, Dim: w}
	scale := float32(size * 0.35)

	left := plc.RunDetector(pigo.Puploc{
		Row:      int(math.Round(ly)),
		Col:      int(math.Round(lx)),
		Scale:    scale,
		Perturbs: 31,
	}, img, 0.0, false)
	right := plc.RunDetector(pigo.Puploc{
		Row:      int(math.Round(ry)),
		Col:      int(math.Round(rx)),
		Scale:    scale,
		Perturbs: 31,
	}, img, 0.0, true)

	plx, ply := float64(left.Col), float64(left.Row)
	prx, pry := float64(right.Col), float64(right.Row)
	leftOK := plx >= cx-0.34*size && plx <= cx-0.10*size && ply >= cy-0.25*size && ply <= cy+0.07*size
	rightOK := prx >= cx+0.10*size && prx <= cx+0.34*size && pry >= cy-0.25*size && pry <= cy+0.07*size
	if leftOK && rightOK && prx > plx+0.22*size && math.Abs(ply-pry) <= 0.18*size {
		return plx, ply, prx, pry
	}
	if leftOK {
		lx, ly = plx, ply
	}
	if rightOK {
		rx, ry = prx, pry
	}
	if rx <= lx+0.22*size {
		return cx - 0.18*size, cy - 0.08*size, cx + 0.18*size, cy - 0.08*size
	}
	return lx, ly, rx, ry
}

func buildDetectionGray(src *image.NRGBA, maxEdge int) ([]uint8, int, int, float64) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	longEdge := w
	if h > longEdge {
		longEdge = h
	}
	scale := 1.0
	detW, detH := w, h
	if longEdge > maxEdge {
		scale = float64(maxEdge) / float64(longEdge)
		detW = max(1, int(math.Round(float64(w)*scale)))
		detH = max(1, int(math.Round(float64(h)*scale)))
	}
	gray := make([]uint8, detW*detH)
	for dy := 0; dy < detH; dy++ {
		sy := min(h-1, int(float64(dy)/scale))
		rowOff := sy * src.Stride
		dstOff := dy * detW
		for dx := 0; dx < detW; dx++ {
			sx := min(w-1, int(float64(dx)/scale))
			i := rowOff + sx*4
			r := int(src.Pix[i])
			g := int(src.Pix[i+1])
			b := int(src.Pix[i+2])
			gray[dstOff+dx] = uint8((299*r + 587*g + 114*b + 500) / 1000)
		}
	}
	return gray, detW, detH, scale
}

func makeSquareGray(gray []uint8, w, h, side, padX, padY int) []uint8 {
	out := make([]uint8, side*side)
	for i := range out {
		out[i] = 128
	}
	for y := 0; y < h; y++ {
		srcOff := y * w
		dstOff := (y+padY)*side + padX
		copy(out[dstOff:dstOff+w], gray[srcOff:srcOff+w])
	}
	return out
}

func mirrorSquareHoriz(sq []uint8, side int) []uint8 {
	out := make([]uint8, len(sq))
	for y := 0; y < side; y++ {
		off := y * side
		for x := 0; x < side; x++ {
			out[off+side-1-x] = sq[off+x]
		}
	}
	return out
}

func suppressOverlappingDetections(dets []pigo.Detection, iouThreshold float64) []pigo.Detection {
	if len(dets) == 0 {
		return nil
	}
	sort.Slice(dets, func(i, j int) bool {
		return dets[i].Q > dets[j].Q
	})
	var kept []pigo.Detection
	for _, cand := range dets {
		dup := false
		for _, prev := range kept {
			if detectionIoU(cand, prev) >= iouThreshold {
				dup = true
				break
			}
		}
		if !dup {
			kept = append(kept, cand)
		}
	}
	return kept
}

func detectionIoU(a, b pigo.Detection) float64 {
	r1, c1, s1 := float64(a.Row), float64(a.Col), float64(a.Scale)
	r2, c2, s2 := float64(b.Row), float64(b.Col), float64(b.Scale)
	overRow := math.Max(0, math.Min(r1+s1/2, r2+s2/2)-math.Max(r1-s1/2, r2-s2/2))
	overCol := math.Max(0, math.Min(c1+s1/2, c2+s2/2)-math.Max(c1-s1/2, c2-s2/2))
	inter := overRow * overCol
	union := s1*s1 + s2*s2 - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}
