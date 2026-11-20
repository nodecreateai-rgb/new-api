package service

import (
	"bytes"
	"image"
	"image/png"
	"math"
)

func commonEncodePNG(buf *bytes.Buffer, img image.Image) error {
	return (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(buf, img)
}

func renderComplianceFaces(dst *image.NRGBA, faces []complianceFace, mode string) {
	orig := cloneNRGBA(dst)
	for _, face := range faces {
		switch mode {
		case "grid":
			renderGridEyes(dst, orig, face)
		case "watercolor":
			renderWatercolorFace(dst, orig, face)
		case "colored-pencil":
			renderColoredPencilFace(dst, orig, face)
		default:
			renderFishnetFace(dst, orig, face)
		}
	}
}

func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	pix := make([]uint8, len(src.Pix))
	copy(pix, src.Pix)
	return &image.NRGBA{
		Pix:    pix,
		Stride: src.Stride,
		Rect:   src.Rect,
	}
}

// renderFishnetFace overlays a tight, stretched diagonal charcoal diamond lattice
// across each detected facial oval so biometric identity features are veiled
// while hairline, neck, clothes, and scene remain untouched.
func renderFishnetFace(dst, orig *image.NRGBA, face complianceFace) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	cx := face.CenterX
	cy := face.CenterY + 0.04*face.Size
	rx := math.Max(face.Size*0.48, 12)
	ry := math.Max(face.Size*0.60, 14)

	period := math.Max(5.0, (2.0*rx)/26.0)
	diagStep := period / math.Sqrt2
	halfStrand := 0.27 * diagStep

	x0, y0, x1, y1 := clampRect(w, h, cx-rx-2, cy-ry-2, cx+rx+2, cy+ry+2)
	for y := y0; y < y1; y++ {
		dy := (float64(y) - cy) / ry
		rowOff := y * dst.Stride
		for x := x0; x < x1; x++ {
			dx := (float64(x) - cx) / rx
			r := math.Hypot(dx, dy)
			if r >= 1.0 {
				continue
			}
			mask := smoothstep(1.0, 0.84, r)

			// Subtle spherical warp simulates stretched fabric over 3D facial planes.
			curve := 1.0 + 0.14*r*r
			wx := (float64(x) - cx) * curve
			wy := (float64(y) - cy) * curve
			u := (wx + wy) / math.Sqrt2
			v := (wx - wy) / math.Sqrt2

			du := math.Abs(positiveMod(u, diagStep) - 0.5*diagStep)
			dv := math.Abs(positiveMod(v, diagStep) - 0.5*diagStep)
			dist := math.Min(du, dv)

			strandAlpha := smoothstep(halfStrand+0.75, halfStrand-0.75, dist)
			// Strengthen eye/brow band coverage slightly to disrupt iris & periocular templates.
			eyeBoost := 0.0
			if dy > -0.45 && dy < -0.02 && math.Abs(dx) < 0.72 {
				eyeBoost = 0.18
			}
			alpha := mask * math.Min(0.95, strandAlpha*0.91+eyeBoost*(1.0-strandAlpha)*0.45)
			if alpha <= 0.005 {
				continue
			}
			idx := rowOff + x*4
			sr, sg, sb := float64(orig.Pix[idx]), float64(orig.Pix[idx+1]), float64(orig.Pix[idx+2])
			// Blend toward deep charcoal (#121216) instead of flat black.
			dst.Pix[idx] = clampU8(sr*(1.0-alpha) + 18.0*alpha)
			dst.Pix[idx+1] = clampU8(sg*(1.0-alpha) + 18.0*alpha)
			dst.Pix[idx+2] = clampU8(sb*(1.0-alpha) + 22.0*alpha)
		}
	}
}

// renderGridEyes covers both eyes, brows, and the upper nose bridge with an
// oriented gray square-cell grid matching the compliance prompt specification.
func renderGridEyes(dst, orig *image.NRGBA, face complianceFace) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	ex := face.RightEyeX - face.LeftEyeX
	ey := face.RightEyeY - face.LeftEyeY
	eyeDist := math.Hypot(ex, ey)
	if eyeDist < face.Size*0.20 {
		eyeDist = face.Size * 0.36
		ex, ey = eyeDist, 0
	}
	cosA := ex / eyeDist
	sinA := ey / eyeDist
	midX := 0.5 * (face.LeftEyeX + face.RightEyeX)
	midY := 0.5 * (face.LeftEyeY + face.RightEyeY)

	halfW := 0.94 * eyeDist
	topH := 0.48 * eyeDist
	botH := 0.34 * eyeDist
	centerOffsetY := 0.5 * (botH - topH)
	halfH := 0.5 * (topH + botH)

	cell := math.Max(4.0, (0.42*eyeDist)/5.5)
	seamHalf := math.Max(0.65, cell*0.14)

	maxSpan := math.Hypot(halfW, halfH) + math.Abs(centerOffsetY) + 4
	x0, y0, x1, y1 := clampRect(w, h, midX-maxSpan, midY-maxSpan, midX+maxSpan, midY+maxSpan)
	for y := y0; y < y1; y++ {
		dy := float64(y) - midY
		rowOff := y * dst.Stride
		for x := x0; x < x1; x++ {
			dx := float64(x) - midX
			// Rotate into head-aligned eye coordinates.
			lx := dx*cosA + dy*sinA
			ly := -dx*sinA + dy*cosA - centerOffsetY

			ax := math.Abs(lx) - (halfW - 4.0)
			ay := math.Abs(ly) - (halfH - 4.0)
			cornerDist := math.Hypot(math.Max(ax, 0), math.Max(ay, 0)) + math.Min(math.Max(ax, ay), 0) - 4.0
			if cornerDist >= 1.5 {
				continue
			}
			mask := smoothstep(1.5, -1.5, cornerDist)

			gx := positiveMod(lx+halfW, cell)
			gy := positiveMod(ly+halfH, cell)
			ci := int(math.Floor((lx + halfW) / cell))
			cj := int(math.Floor((ly + halfH) / cell))

			// Sample quantized cell center back in image space to anchor lighting subtly.
			cxLocal := (float64(ci)+0.5)*cell - halfW
			cyLocal := (float64(cj)+0.5)*cell - halfH + centerOffsetY
			sampleX := clampInt(int(math.Round(midX+cxLocal*cosA-cyLocal*sinA)), 0, w-1)
			sampleY := clampInt(int(math.Round(midY+cxLocal*sinA+cyLocal*cosA)), 0, h-1)
			sIdx := sampleY*orig.Stride + sampleX*4
			cellLum := 0.299*float64(orig.Pix[sIdx]) + 0.587*float64(orig.Pix[sIdx+1]) + 0.114*float64(orig.Pix[sIdx+2])

			// High-opacity medium gray with subtle checker contrast & light gray seams.
			checker := ((ci + cj) & 1) * 16
			baseGray := 102.0 + 0.14*(cellLum-128.0) + float64(checker) - 8.0
			seamX := math.Min(gx, cell-gx)
			seamY := math.Min(gy, cell-gy)
			seamDist := math.Min(seamX, seamY)
			seamMix := smoothstep(seamHalf+0.5, math.Max(0, seamHalf-0.5), seamDist)

			fillGray := baseGray*(1.0-seamMix) + 178.0*seamMix
			alpha := mask * 0.93

			idx := rowOff + x*4
			sr, sg, sb := float64(orig.Pix[idx]), float64(orig.Pix[idx+1]), float64(orig.Pix[idx+2])
			dst.Pix[idx] = clampU8(sr*(1.0-alpha) + fillGray*alpha)
			dst.Pix[idx+1] = clampU8(sg*(1.0-alpha) + fillGray*alpha)
			dst.Pix[idx+2] = clampU8(sb*(1.0-alpha) + (fillGray+3.0)*alpha)
		}
	}
}

// renderWatercolorFace applies face-local geometric biometric displacement,
// integral-table Kuwahara painterly smoothing, and wet pigment edge pooling
// exclusively inside a feathered oval over the detected face.
func renderWatercolorFace(dst, orig *image.NRGBA, face complianceFace) {
	bw, bh, x0, y0, cxLocal, cyLocal, rx, ry, ok := facePaintRegion(dst, face)
	if !ok {
		return
	}
	warped := buildWarpedFacePatch(orig, x0, y0, bw, bh, face)
	radius := clampInt(int(math.Round(face.Size*0.025)), 2, 7)
	kuwahara := applyKuwaharaRGB(warped, bw, bh, radius)

	for ly := 1; ly < bh-1; ly++ {
		dy := (float64(ly) - cyLocal) / ry
		gy := y0 + ly
		rowOff := gy * dst.Stride
		for lx := 1; lx < bw-1; lx++ {
			dx := (float64(lx) - cxLocal) / rx
			r := math.Hypot(dx, dy)
			if r >= 1.0 {
				continue
			}
			mask := smoothstep(1.0, 0.72, r) * 0.92
			pIdx := (ly*bw + lx) * 3
			pr, pg, pb := float64(kuwahara[pIdx]), float64(kuwahara[pIdx+1]), float64(kuwahara[pIdx+2])

			// Soft tonal quantization + watercolor pigment lift + wet edge granulation.
			edge := localGradientNorm(kuwahara, bw, lx, ly)
			paper := math.Sin(float64(x0+lx)*0.19)*math.Cos(float64(y0+ly)*0.17)*6.0 + (hash2D(x0+lx, y0+ly)-0.5)*7.0
			edgeDark := math.Min(edge*0.32, 34.0)

			pr = quantizeSoft(pr, 24.0, 0.35) + paper - edgeDark*0.65 + 4.0
			pg = quantizeSoft(pg, 24.0, 0.35) + paper*0.9 - edgeDark*0.85
			pb = quantizeSoft(pb, 24.0, 0.35) + paper*0.8 - edgeDark*0.75 + 2.0

			idx := rowOff + (x0+lx)*4
			sr, sg, sb := float64(orig.Pix[idx]), float64(orig.Pix[idx+1]), float64(orig.Pix[idx+2])
			dst.Pix[idx] = clampU8(sr*(1.0-mask) + pr*mask)
			dst.Pix[idx+1] = clampU8(sg*(1.0-mask) + pg*mask)
			dst.Pix[idx+2] = clampU8(sb*(1.0-mask) + pb*mask)
		}
	}
}

// renderColoredPencilFace applies face-local geometric biometric displacement,
// restrained ochre/umber colored-pencil toning, Sobel sketch contours, and
// directional multi-angle cross-hatching inside a feathered facial oval.
func renderColoredPencilFace(dst, orig *image.NRGBA, face complianceFace) {
	bw, bh, x0, y0, cxLocal, cyLocal, rx, ry, ok := facePaintRegion(dst, face)
	if !ok {
		return
	}
	warped := buildWarpedFacePatch(orig, x0, y0, bw, bh, face)
	smooth := applyKuwaharaRGB(warped, bw, bh, clampInt(int(math.Round(face.Size*0.015)), 2, 4))

	strokeFreq := math.Max(1.8, face.Size/75.0)
	for ly := 1; ly < bh-1; ly++ {
		dy := (float64(ly) - cyLocal) / ry
		gy := y0 + ly
		rowOff := gy * dst.Stride
		for lx := 1; lx < bw-1; lx++ {
			dx := (float64(lx) - cxLocal) / rx
			r := math.Hypot(dx, dy)
			if r >= 1.0 {
				continue
			}
			mask := smoothstep(1.0, 0.72, r) * 0.93
			pIdx := (ly*bw + lx) * 3
			pr, pg, pb := float64(smooth[pIdx]), float64(smooth[pIdx+1]), float64(smooth[pIdx+2])

			// Desaturate slightly toward warm ochre / gray-violet shadow pencil gamut.
			lum := 0.299*pr + 0.587*pg + 0.114*pb
			pr = pr*0.74 + (lum+7.0)*0.26
			pg = pg*0.74 + (lum+2.0)*0.26
			pb = pb*0.74 + (lum-6.0)*0.26

			edge := localGradientNorm(smooth, bw, lx, ly)
			// Directional pencil cross-hatching weighted by tonal darkness.
			px, py := float64(x0+lx)/strokeFreq, float64(y0+ly)/strokeFreq
			h1 := math.Sin(px*1.7 + py*0.95)
			h2 := math.Sin(-px*0.95 + py*1.85 + 1.3)
			darkness := math.Max(0, 1.0-lum/235.0)

			grain := (hash2D(x0+lx, y0+ly) - 0.5) * 16.0 * (0.4 + 0.6*(1.0-darkness))
			hatch := 0.0
			if lum < 195 {
				hatch += math.Max(0, h1) * darkness * 24.0
			}
			if lum < 130 {
				hatch += math.Max(0, h2) * darkness * 22.0
			}
			contour := math.Min(edge*0.45, 42.0)

			rr := pr + grain - hatch*0.92 - contour*0.85
			gg := pg + grain - hatch*0.96 - contour*0.92
			bb := pb + grain - hatch*0.88 - contour*0.82

			idx := rowOff + (x0+lx)*4
			sr, sg, sb := float64(orig.Pix[idx]), float64(orig.Pix[idx+1]), float64(orig.Pix[idx+2])
			dst.Pix[idx] = clampU8(sr*(1.0-mask) + rr*mask)
			dst.Pix[idx+1] = clampU8(sg*(1.0-mask) + gg*mask)
			dst.Pix[idx+2] = clampU8(sb*(1.0-mask) + bb*mask)
		}
	}
}

func facePaintRegion(img *image.NRGBA, face complianceFace) (bw, bh, x0, y0 int, cxLocal, cyLocal, rx, ry float64, ok bool) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	cx := face.CenterX
	cy := face.CenterY + 0.04*face.Size
	rx = math.Max(face.Size*0.54, 14)
	ry = math.Max(face.Size*0.66, 16)
	var x1, y1 int
	x0, y0, x1, y1 = clampRect(w, h, cx-rx-8, cy-ry-8, cx+rx+8, cy+ry+8)
	bw = x1 - x0
	bh = y1 - y0
	if bw < 8 || bh < 8 {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}
	return bw, bh, x0, y0, cx - float64(x0), cy - float64(y0), rx, ry, true
}

// buildWarpedFacePatch samples orig inside [x0..x0+bw, y0..y0+bh] while applying a
// smooth C1 biometric displacement to eye spacing, nasal bridge, and mouth ratios.
func buildWarpedFacePatch(orig *image.NRGBA, x0, y0, bw, bh int, face complianceFace) []uint8 {
	patch := make([]uint8, bw*bh*3)
	cx := face.CenterX
	cy := face.CenterY + 0.04*face.Size
	rx := math.Max(face.Size*0.54, 14)
	ry := math.Max(face.Size*0.66, 16)
	amp := math.Max(2.0, face.Size*0.042)

	for ly := 0; ly < bh; ly++ {
		gy := float64(y0 + ly)
		ny := (gy - cy) / ry
		rowOff := ly * bw * 3
		for lx := 0; lx < bw; lx++ {
			gx := float64(x0 + lx)
			nx := (gx - cx) / rx
			r := math.Hypot(nx, ny)
			sx, sy := gx, gy
			if r < 1.0 {
				env := smoothstep(1.0, 0.25, r)
				// Smooth multi-harmonic landmark ratio warp (zero discontinuity at boundary).
				dx := amp * env * math.Sin(nx*math.Pi*1.5) * math.Cos(ny*math.Pi*0.75)
				dy := amp * env * 0.75 * math.Sin(ny*math.Pi*2.0) * math.Cos(nx*math.Pi*0.65)
				sx += dx
				sy += dy
			}
			rVal, gVal, bVal := sampleBilinearRGB(orig, sx, sy)
			o := rowOff + lx*3
			patch[o] = rVal
			patch[o+1] = gVal
			patch[o+2] = bVal
		}
	}
	return patch
}

// applyKuwaharaRGB computes the 4-quadrant edge-preserving Kuwahara filter in
// O(1) time per pixel using integral image prefix sum tables.
func applyKuwaharaRGB(src []uint8, w, h, radius int) []uint8 {
	out := make([]uint8, len(src))
	pw := w + 1
	size := pw * (h + 1)
	sumR := make([]int64, size)
	sumG := make([]int64, size)
	sumB := make([]int64, size)
	sqSum := make([]int64, size)

	for y := 0; y < h; y++ {
		var rowR, rowG, rowB, rowSq int64
		for x := 0; x < w; x++ {
			si := (y*w + x) * 3
			r, g, b := int64(src[si]), int64(src[si+1]), int64(src[si+2])
			lum := (299*r + 587*g + 114*b) / 1000
			rowR += r
			rowG += g
			rowB += b
			rowSq += lum*lum*2 + (r*r+g*g+b*b)/3
			cur := (y+1)*pw + (x + 1)
			above := y*pw + (x + 1)
			sumR[cur] = sumR[above] + rowR
			sumG[cur] = sumG[above] + rowG
			sumB[cur] = sumB[above] + rowB
			sqSum[cur] = sqSum[above] + rowSq
		}
	}
	sumLum := func(tx0, ty0, tx1, ty1 int) (meanR, meanG, meanB, variance float64) {
		n := float64((tx1 - tx0) * (ty1 - ty0))
		if n <= 0 {
			return 0, 0, 0, math.MaxFloat64
		}
		a := ty0*pw + tx0
		b := ty0*pw + tx1
		c := ty1*pw + tx0
		d := ty1*pw + tx1
		sr := float64(sumR[d] - sumR[b] - sumR[c] + sumR[a])
		sg := float64(sumG[d] - sumG[b] - sumG[c] + sumG[a])
		sb := float64(sumB[d] - sumB[b] - sumB[c] + sumB[a])
		ss := float64(sqSum[d] - sqSum[b] - sqSum[c] + sqSum[a])
		meanL := (0.299*sr + 0.587*sg + 0.114*sb) / n
		v := ss/n - 3.0*meanL*meanL
		return sr / n, sg / n, sb / n, v
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			quads := [4][4]int{
				{max(0, x-radius), max(0, y-radius), x + 1, y + 1},
				{x, max(0, y-radius), min(w, x+radius+1), y + 1},
				{max(0, x-radius), y, x + 1, min(h, y+radius+1)},
				{x, y, min(w, x+radius+1), min(h, y+radius+1)},
			}
			bestVar := math.MaxFloat64
			var br, bg, bb float64
			for _, q := range quads {
				mr, mg, mb, v := sumLum(q[0], q[1], q[2], q[3])
				if v < bestVar {
					bestVar = v
					br, bg, bb = mr, mg, mb
				}
			}
			di := (y*w + x) * 3
			out[di] = clampU8(br)
			out[di+1] = clampU8(bg)
			out[di+2] = clampU8(bb)
		}
	}
	return out
}

func sampleBilinearRGB(img *image.NRGBA, x, y float64) (uint8, uint8, uint8) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if x < 0 {
		x = 0
	} else if x > float64(w-1) {
		x = float64(w - 1)
	}
	if y < 0 {
		y = 0
	} else if y > float64(h-1) {
		y = float64(h - 1)
	}
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := min(w-1, x0+1)
	y1 := min(h-1, y0+1)
	fx := x - float64(x0)
	fy := y - float64(y0)
	i00 := y0*img.Stride + x0*4
	i10 := y0*img.Stride + x1*4
	i01 := y1*img.Stride + x0*4
	i11 := y1*img.Stride + x1*4
	w00 := (1 - fx) * (1 - fy)
	w10 := fx * (1 - fy)
	w01 := (1 - fx) * fy
	w11 := fx * fy
	r := float64(img.Pix[i00])*w00 + float64(img.Pix[i10])*w10 + float64(img.Pix[i01])*w01 + float64(img.Pix[i11])*w11
	g := float64(img.Pix[i00+1])*w00 + float64(img.Pix[i10+1])*w10 + float64(img.Pix[i01+1])*w01 + float64(img.Pix[i11+1])*w11
	b := float64(img.Pix[i00+2])*w00 + float64(img.Pix[i10+2])*w10 + float64(img.Pix[i01+2])*w01 + float64(img.Pix[i11+2])*w11
	return clampU8(r), clampU8(g), clampU8(b)
}

func localGradientNorm(rgb []uint8, w, x, y int) float64 {
	lumAt := func(px, py int) float64 {
		i := (py*w + px) * 3
		return 0.299*float64(rgb[i]) + 0.587*float64(rgb[i+1]) + 0.114*float64(rgb[i+2])
	}
	gx := (lumAt(x+1, y-1) + 2*lumAt(x+1, y) + lumAt(x+1, y+1)) - (lumAt(x-1, y-1) + 2*lumAt(x-1, y) + lumAt(x-1, y+1))
	gy := (lumAt(x-1, y+1) + 2*lumAt(x, y+1) + lumAt(x+1, y+1)) - (lumAt(x-1, y-1) + 2*lumAt(x, y-1) + lumAt(x+1, y-1))
	return math.Hypot(gx, gy) * 0.25
}

func quantizeSoft(val, step, strength float64) float64 {
	q := math.Round(val/step) * step
	return val*(1.0-strength) + q*strength
}

func hash2D(x, y int) float64 {
	n := uint32(x)*374761393 + uint32(y)*668265263
	n = (n ^ (n >> 13)) * 1274126177
	return float64(n^(n>>16)) / float64(math.MaxUint32)
}

func positiveMod(a, m float64) float64 {
	v := math.Mod(a, m)
	if v < 0 {
		v += m
	}
	return v
}

func smoothstep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func clampRect(w, h int, fx0, fy0, fx1, fy1 float64) (int, int, int, int) {
	x0 := clampInt(int(math.Floor(fx0)), 0, w)
	y0 := clampInt(int(math.Floor(fy0)), 0, h)
	x1 := clampInt(int(math.Ceil(fx1)), 0, w)
	y1 := clampInt(int(math.Ceil(fy1)), 0, h)
	return x0, y0, x1, y1
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampU8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
