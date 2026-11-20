package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func disableSSRFForLocalTest(t *testing.T) {
	t.Helper()
	fs := system_setting.GetFetchSetting()
	orig := *fs
	fs.EnableSSRFProtection = false
	fs.AllowPrivateIp = true
	t.Cleanup(func() {
		*fs = orig
	})
}

func loadLenaBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/lena.jpg")
	if err != nil {
		t.Fatalf("read testdata/lena.jpg: %v", err)
	}
	return raw
}

func loadLenaDataURL(t *testing.T) string {
	t.Helper()
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(loadLenaBytes(t))
}

func makeNonFacePatternPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			c := color.NRGBA{R: 235, G: 238, B: 242, A: 255}
			if ((x/32)+(y/32))%2 == 0 {
				c = color.NRGBA{R: 52, G: 84, B: 132, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInspectComplianceCountsDistinctImagesAndPrice(t *testing.T) {
	body := []byte(`{
		"model":"seedance-2.0",
		"prompt":"walk",
		"compliance_enabled":"true",
		"compliance_mode":"colored_pencil",
		"image_refs":["https://cdn.example/a.png","https://cdn.example/a.png"],
		"image_url":{"url":"https://cdn.example/b.png"}
	}`)
	opt, n, err := InspectCompliance(body, "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if !opt.Enabled || opt.Mode != "colored-pencil" || n != 2 {
		t.Fatalf("opt=%+v images=%d", opt, n)
	}
	if got := opt.Quota(n, 1); got != 0 {
		t.Fatalf("compliance preprocessing must be free, got quota=%d", got)
	}
	if opt.Quota(n, 0) != 0 {
		t.Fatal("zero group ratio must not charge")
	}
}

func TestInspectComplianceReadsMetadataAndRejectsMode(t *testing.T) {
	body := []byte(`{"metadata":{"compliance_enabled":true,"compliance_mode":"nope"},"image":"https://cdn.example/a.png"}`)
	if _, _, err := InspectCompliance(body, "application/json"); err == nil || !strings.Contains(err.Error(), "invalid compliance_mode") {
		t.Fatalf("err=%v", err)
	}
	off := []byte(`{"compliance_enabled":false,"image_refs":["https://cdn.example/a.png"]}`)
	opt, n, err := InspectCompliance(off, "application/json")
	if err != nil || opt.Enabled || n != 0 {
		t.Fatalf("opt=%+v n=%d err=%v", opt, n, err)
	}
}

func TestApplyComplianceMaterialsLocalPortraitRewritesAndStripsFlags(t *testing.T) {
	disableSSRFForLocalTest(t)
	lena := loadLenaBytes(t)
	var fetches int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/portrait.jpg" {
			fetches++
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(lena)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	portraitURL := srv.URL + "/portrait.jpg"
	body := []byte(`{
		"model":"seedance-2.0-480p-c7",
		"prompt":"keep me",
		"compliance_enabled":true,
		"compliance_mode":"grid",
		"prompt_compliance_enabled":true,
		"image_refs":["` + portraitURL + `","` + portraitURL + `"],
		"image_url":{"url":"` + portraitURL + `"}
	}`)
	out, ct, err := ApplyComplianceMaterials(context.Background(), body, "application/json", ComplianceMaterial{Enabled: true, Mode: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "application/json" {
		t.Fatalf("content type %s", ct)
	}
	if fetches != 1 {
		t.Fatalf("expected deduplicated 1 fetch, got %d", fetches)
	}

	var payload map[string]any
	if err := common.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["compliance_enabled"]; ok {
		t.Fatalf("compliance flag still present: %#v", payload)
	}
	if _, ok := payload["compliance_mode"]; ok {
		t.Fatal("compliance mode still present")
	}
	prompt, _ := payload["prompt"].(string)
	if payload["prompt_compliance_enabled"] != true || !strings.Contains(prompt, "keep me") || !strings.HasPrefix(prompt, "【最高优先级·首帧前强制去网格】") || !strings.Contains(prompt, "【生成视频】keep me") {
		t.Fatalf("prompt was not carried for grid mode: %#v", payload["prompt"])
	}
	refs, _ := payload["image_refs"].([]any)
	if len(refs) != 2 || !strings.HasPrefix(refs[0].(string), "data:image/") || refs[0] != refs[1] {
		t.Fatalf("refs=%#v", refs)
	}
	imageURL, _ := payload["image_url"].(map[string]any)
	if imageURL["url"] != refs[0] {
		t.Fatalf("image_url=%#v", imageURL)
	}

	rawOut, _, err := decodeDataURL(refs[0].(string))
	if err != nil {
		t.Fatal(err)
	}
	srcImg, _, _, err := decodeComplianceNRGBA(lena)
	if err != nil {
		t.Fatal(err)
	}
	dstImg, _, _, err := decodeComplianceNRGBA(rawOut)
	if err != nil {
		t.Fatal(err)
	}
	if srcImg.Bounds() != dstImg.Bounds() {
		t.Fatalf("bounds changed: src=%v dst=%v", srcImg.Bounds(), dstImg.Bounds())
	}
	faces, err := detectComplianceFaces(srcImg)
	if err != nil || len(faces) == 0 {
		t.Fatalf("expected face in lena: faces=%d err=%v", len(faces), err)
	}
	changedFace, changedCorner := countDiffPixels(srcImg, dstImg, faces[0])
	if changedFace == 0 {
		t.Fatalf("expected eye/face pixels to be modified")
	}
	if changedCorner != 0 {
		t.Fatalf("expected far background corners to stay untouched, changed=%d", changedCorner)
	}
}

func TestLocalComplianceAllModesOnPortraitAndNoFace(t *testing.T) {
	lenaURL := loadLenaDataURL(t)
	srcRaw, _, _ := decodeDataURL(lenaURL)
	srcImg, _, _, err := decodeComplianceNRGBA(srcRaw)
	if err != nil {
		t.Fatal(err)
	}
	faces, err := detectComplianceFaces(srcImg)
	if err != nil || len(faces) == 0 {
		t.Fatalf("detectComplianceFaces failed: faces=%d err=%v", len(faces), err)
	}

	for _, mode := range []string{"fishnet", "grid", "watercolor", "colored-pencil"} {
		outURL, modified, err := editComplianceImageLocal(context.Background(), mode, lenaURL)
		if err != nil {
			t.Fatalf("mode %s error: %v", mode, err)
		}
		if !modified || !strings.HasPrefix(outURL, "data:image/") {
			t.Fatalf("mode %s expected treated data url, got modified=%v", mode, modified)
		}
		outRaw, _, err := decodeDataURL(outURL)
		if err != nil {
			t.Fatalf("mode %s decodeDataURL: %v", mode, err)
		}
		dstImg, _, _, err := decodeComplianceNRGBA(outRaw)
		if err != nil {
			t.Fatalf("mode %s decode output: %v", mode, err)
		}
		changedFace, changedCorner := countDiffPixels(srcImg, dstImg, faces[0])
		if changedFace < 500 {
			t.Fatalf("mode %s changed too few face pixels (%d)", mode, changedFace)
		}
		if changedCorner != 0 {
			t.Fatalf("mode %s leaked outside face region (%d corner pixels)", mode, changedCorner)
		}
		if mode == "fishnet" || mode == "grid" {
			postFaces, perr := detectComplianceFaces(dstImg)
			t.Logf("mode=%s changedFace=%d preQ=%.2f postFaces=%d err=%v", mode, changedFace, faces[0].Confidence, len(postFaces), perr)
		}
	}

	// Verify non-face geometric pattern passes through completely unchanged.
	patternPNG := makeNonFacePatternPNG(t)
	patternURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(patternPNG)
	outURL, modified, err := editComplianceImageLocal(context.Background(), "fishnet", patternURL)
	if err != nil {
		t.Fatal(err)
	}
	if modified || outURL != patternURL {
		t.Fatalf("expected non-face image untouched, got modified=%v", modified)
	}
}

func TestLocalComplianceMultipartMixedPortraitAndNoFace(t *testing.T) {
	lena := loadLenaBytes(t)
	pattern := makeNonFacePatternPNG(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "seedance-2.0-480p-c7")
	_ = writer.WriteField("prompt", "hello world")
	_ = writer.WriteField("compliance_enabled", "true")
	_ = writer.WriteField("compliance_mode", "fishnet")

	p1, err := writer.CreateFormFile("image", "portrait.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = p1.Write(lena)
	p2, err := writer.CreateFormFile("extra_images", "pattern.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = p2.Write(pattern)
	_ = writer.Close()

	outBody, outCT, err := ApplyComplianceMaterials(context.Background(), body.Bytes(), writer.FormDataContentType(), ComplianceMaterial{Enabled: true, Mode: "fishnet"})
	if err != nil {
		t.Fatal(err)
	}
	form, err := complianceReadForm(outCT, outBody)
	if err != nil {
		t.Fatal(err)
	}
	defer form.RemoveAll()

	if len(form.Value["compliance_enabled"]) != 0 || len(form.Value["compliance_mode"]) != 0 {
		t.Fatalf("expected compliance fields stripped: %#v", form.Value)
	}
	prompt := firstFormValue(form, "prompt")
	if !strings.HasPrefix(prompt, "【整脸去网纱】") || !strings.Contains(prompt, "hello world") {
		t.Fatalf("unexpected multipart prompt: %s", prompt)
	}
	if len(form.File["image"]) != 1 || len(form.File["extra_images"]) != 1 {
		t.Fatalf("missing uploaded files: %#v", form.File)
	}
	fPortrait, _ := complianceFileDataURL(form.File["image"][0])
	if fPortrait == loadLenaDataURL(t) {
		t.Fatal("expected portrait multipart file to be edited")
	}
	fPattern, _ := complianceFileDataURL(form.File["extra_images"][0])
	if fPattern != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(pattern) {
		t.Fatal("expected non-face multipart file to stay byte-identical")
	}
}

func TestLocalComplianceHandlesExifOrientationAndVerticalFrame(t *testing.T) {
	lenaRaw := loadLenaBytes(t)
	lenaImg, _, err := image.Decode(bytes.NewReader(lenaRaw))
	if err != nil {
		t.Fatal(err)
	}
	nrgba, _ := imageToNRGBA(lenaImg)

	// 1. Tall rectangular 9:16 portrait canvas must never trigger bounds panics during rotated Pigo passes.
	tall := image.NewNRGBA(image.Rect(0, 0, 512, 910))
	for y := 0; y < 512; y++ {
		copy(tall.Pix[(y+180)*tall.Stride:((y+180)*tall.Stride)+512*4], nrgba.Pix[y*nrgba.Stride:y*nrgba.Stride+512*4])
	}
	tallFaces, err := detectComplianceFaces(tall)
	if err != nil || len(tallFaces) == 0 {
		t.Fatalf("vertical frame face detection failed: faces=%d err=%v", len(tallFaces), err)
	}

	// 2. EXIF Orientation=6 (90 deg CW display): rotate raw pixels 270 deg CW first and attach EXIF tag 6.
	rot270 := applyExifOrientationNRGBA(nrgba, 8)
	var jpgBuf bytes.Buffer
	if err := jpeg.Encode(&jpgBuf, rot270, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	withExif := injectTestJPEGExifOrientation(jpgBuf.Bytes(), 6)
	exifURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(withExif)
	outURL, modified, err := editComplianceImageLocal(context.Background(), "watercolor", exifURL)
	if err != nil {
		t.Fatal(err)
	}
	if !modified || outURL == exifURL {
		t.Fatal("expected EXIF orientation=6 portrait to be uprighted and treated")
	}
}

func TestSeedance25C5SkipsComplianceEvenWhenEnabled(t *testing.T) {
	if !CompliancePassthrough("seedance-2.0-fast-720p-c5") || !CompliancePassthrough("seedance-2.5-720p-c5") || !CompliancePassthrough("seedance-2.5-480p-c5") || !CompliancePassthrough("seedance-2.5-720p-c6") || !CompliancePassthrough("seedance-2.5-flash-480p-c5") || !CompliancePassthrough("seedance-2.5-flash-720p-c5") {
		t.Fatal("c5 must skip compliance")
	}
	if !CompliancePassthrough("seedance-2.5-720p") || !CompliancePassthrough("seedance-2.5-flash-720p") {
		t.Fatal("seedance-2.5-flash-720p must skip compliance")
	}
	if CompliancePassthrough("seedance-2.5-720p-c4") {
		t.Fatal("c4 still uses gateway compliance")
	}
	body := []byte(`{"model":"seedance2.5-V","prompt":"角色走进房间","compliance_enabled":true,"compliance_mode":"grid","image_refs":["https://cdn.example/a.png"]}`)
	out, _, err := ApplyComplianceMaterials(context.Background(), body, "application/json", ComplianceMaterial{Passthrough: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("out=%s", out)
	}
	opt := ComplianceMaterial{Enabled: true, Mode: "grid", Passthrough: true}
	if opt.Quota(8, 1) != 0 {
		t.Fatal("passthrough must not add image credits")
	}
}

func TestApplyComplianceMaterialsLeavesPlainRequestsUntouched(t *testing.T) {
	body := []byte(`{"prompt":"walk","image_refs":["https://cdn.example/a.png"],"compliance_enabled":false}`)
	out, _, err := ApplyComplianceMaterials(context.Background(), body, "application/json", ComplianceMaterial{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("out=%s", out)
	}
}

func countDiffPixels(a, b *image.NRGBA, face complianceFace) (faceChanged, cornerChanged int) {
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	fx0 := max(0, int(face.CenterX-face.Size*0.65))
	fx1 := min(w, int(face.CenterX+face.Size*0.65))
	fy0 := max(0, int(face.CenterY-face.Size*0.70))
	fy1 := min(h, int(face.CenterY+face.Size*0.75))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ia := y*a.Stride + x*4
			ib := y*b.Stride + x*4
			delta := absIntDiff(int(a.Pix[ia])-int(b.Pix[ib])) +
				absIntDiff(int(a.Pix[ia+1])-int(b.Pix[ib+1])) +
				absIntDiff(int(a.Pix[ia+2])-int(b.Pix[ib+2]))
			if x >= fx0 && x < fx1 && y >= fy0 && y < fy1 {
				if delta > 18 {
					faceChanged++
				}
			} else if (x < 16 || x >= w-16) && (y < 16 || y >= h-16) {
				if delta > 4 {
					cornerChanged++
				}
			}
		}
	}
	return faceChanged, cornerChanged
}

func absIntDiff(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func injectTestJPEGExifOrientation(raw []byte, orientation uint16) []byte {
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return raw
	}
	// Minimal APP1 + Exif\x00\x00 + little-endian TIFF with 1 IFD entry (Orientation=0x0112).
	payload := make([]byte, 6+8+2+12+4)
	copy(payload[0:6], []byte("Exif\x00\x00"))
	copy(payload[6:8], []byte("II"))
	binary.LittleEndian.PutUint16(payload[8:10], 0x002A)
	binary.LittleEndian.PutUint32(payload[10:14], 8) // IFD0 offset
	binary.LittleEndian.PutUint16(payload[14:16], 1) // 1 entry
	binary.LittleEndian.PutUint16(payload[16:18], 0x0112)
	binary.LittleEndian.PutUint16(payload[18:20], 3) // SHORT
	binary.LittleEndian.PutUint32(payload[20:24], 1)
	binary.LittleEndian.PutUint16(payload[24:26], orientation)

	seg := make([]byte, 4+len(payload))
	seg[0] = 0xFF
	seg[1] = 0xE1
	binary.BigEndian.PutUint16(seg[2:4], uint16(2+len(payload)))
	copy(seg[4:], payload)

	var out bytes.Buffer
	out.Write(raw[:2])
	out.Write(seg)
	out.Write(raw[2:])
	return out.Bytes()
}
