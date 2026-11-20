package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

// complianceImageCredit is 0: reference-image compliance preprocessing is
// built-in and free (matching video upscaling).
const complianceImageCredit = 0.0

// ComplianceMaterial is the gateway decision for a video request.
// Enabled means reference images are edited locally here before the video
// upstream sees them.
// Passthrough means the selected model already accepts real faces, so an
// enabled flag must not edit images, add a cleanup prompt, or add a charge.
type ComplianceMaterial struct {
	Enabled     bool
	Mode        string
	Passthrough bool
}

func (m ComplianceMaterial) Quota(images int, groupRatio float64) int {
	if complianceImageCredit <= 0 || m.Passthrough || !m.Enabled || images <= 0 || groupRatio <= 0 {
		return 0
	}
	return images * int(complianceImageCredit*common.QuotaPerUnit*groupRatio)
}

// CompliancePassthrough reports models that already accept real faces.
// seedance-2.5-720p and the xingapi 2.5 aliases are native, so compliance must
// not run even when the request turns it on.
func CompliancePassthrough(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "seedance-2.5-720p", "seedance-2.5-flash-720p", "seedance-2.0-fast-720p-c5", "seedance-2.5-720p-c5", "seedance-2.5-480p-c5", "seedance-2.5-720p-c6", "seedance-2.5-flash-480p-c5", "seedance-2.5-flash-720p-c5":
		return true
	default:
		return false
	}
}

var complianceImageKeys = map[string]struct{}{
	"image": {}, "image_url": {}, "images": {}, "image_urls": {},
	"image_refs": {}, "reference_image": {}, "reference_images": {},
	"input_reference": {}, "first_frame": {}, "last_frame": {}, "extra_images": {},
}

// InspectCompliance reads compliance_enabled / compliance_mode (including the
// eye_mask aliases and metadata) and counts distinct image materials.
func InspectCompliance(body []byte, contentType string) (ComplianceMaterial, int, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return ComplianceMaterial{}, 0, nil
	}
	if complianceBodyIsJSON(contentType, body) {
		var payload map[string]any
		if err := common.Unmarshal(body, &payload); err != nil {
			return ComplianceMaterial{}, 0, nil
		}
		opt, err := complianceFromMap(payload)
		if err != nil || !opt.Enabled {
			return opt, 0, err
		}
		return opt, len(uniqueStrings(collectImageValues(payload, false))), nil
	}
	if !strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		return ComplianceMaterial{}, 0, nil
	}
	form, err := complianceReadForm(contentType, body)
	if err != nil {
		return ComplianceMaterial{}, 0, err
	}
	defer form.RemoveAll()
	opt, err := complianceFromForm(form)
	if err != nil || !opt.Enabled {
		return opt, 0, err
	}
	return opt, len(complianceFormImages(form)), nil
}

func InspectComplianceFromContext(c *gin.Context) (ComplianceMaterial, int, error) {
	if c == nil || c.Request == nil {
		return ComplianceMaterial{}, 0, nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return ComplianceMaterial{}, 0, nil
	}
	body, err := storage.Bytes()
	if err != nil {
		return ComplianceMaterial{}, 0, nil
	}
	return InspectCompliance(body, c.GetHeader("Content-Type"))
}

// ApplyComplianceMaterials edits each reference image locally and rewrites
// the upstream body so the video service receives the treated materials with
// compliance flags removed.
func ApplyComplianceMaterials(ctx context.Context, body []byte, contentType string, opt ComplianceMaterial) ([]byte, string, error) {
	if opt.Passthrough {
		return body, contentType, nil
	}
	if !opt.Enabled {
		detected, _, err := InspectCompliance(body, contentType)
		if err != nil {
			return nil, contentType, err
		}
		if !detected.Enabled {
			return body, contentType, nil
		}
		opt = detected
	}
	normalized, err := normalizeComplianceMode(opt.Mode)
	if err != nil {
		return nil, contentType, err
	}
	opt.Mode = normalized

	if complianceBodyIsJSON(contentType, body) {
		return applyComplianceJSON(ctx, body, opt)
	}
	if strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		return applyComplianceMultipart(ctx, body, contentType, opt)
	}
	return body, contentType, nil
}

func applyComplianceJSON(ctx context.Context, body []byte, opt ComplianceMaterial) ([]byte, string, error) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return body, "application/json", nil
	}
	images := uniqueStrings(collectImageValues(payload, false))
	if len(images) == 0 {
		return body, "application/json", nil
	}
	replaced, edited, err := editComplianceImages(ctx, opt.Mode, images)
	if err != nil {
		return nil, "application/json", err
	}
	rewritten, _ := replaceImageValues(payload, false, replaced).(map[string]any)
	if edited > 0 {
		applyCompliancePrompt(rewritten, opt.Mode)
	}
	stripComplianceFlags(rewritten)
	out, err := common.Marshal(rewritten)
	if err != nil {
		return nil, "application/json", err
	}
	return out, "application/json", nil
}

func editComplianceImages(ctx context.Context, mode string, images []string) (map[string]string, int, error) {
	out := make(map[string]string, len(images))
	edited := 0
	for i, src := range images {
		if err := ctx.Err(); err != nil {
			return nil, edited, err
		}
		dataURL, modified, err := editComplianceImageLocal(ctx, mode, src)
		if err != nil {
			return nil, edited, fmt.Errorf("compliance image %d failed: %w", i+1, err)
		}
		out[src] = dataURL
		if modified {
			edited++
		}
	}
	logger.LogInfo(ctx, fmt.Sprintf("compliance: locally inspected %d image(s), treated %d image(s) with mode %s", len(images), edited, mode))
	return out, edited, nil
}

func asComplianceString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func complianceFromMap(payload map[string]any) (ComplianceMaterial, error) {
	enabled, present, err := flexibleComplianceBool(payload["compliance_enabled"])
	if err != nil {
		return ComplianceMaterial{}, err
	}
	if !present {
		enabled, present, err = flexibleComplianceBool(payload["eye_mask_enabled"])
		if err != nil {
			return ComplianceMaterial{}, err
		}
	}
	mode := firstComplianceString(payload["compliance_mode"], payload["eye_mask_mode"])
	if meta, ok := payload["metadata"].(map[string]any); ok {
		if !present {
			enabled, present, err = flexibleComplianceBool(meta["compliance_enabled"])
			if err != nil {
				return ComplianceMaterial{}, err
			}
		}
		if !present {
			enabled, _, err = flexibleComplianceBool(meta["eye_mask_enabled"])
			if err != nil {
				return ComplianceMaterial{}, err
			}
		}
		if mode == "" {
			mode = firstComplianceString(meta["compliance_mode"], meta["eye_mask_mode"])
		}
	}
	if !enabled {
		return ComplianceMaterial{}, nil
	}
	mode, err = normalizeComplianceMode(mode)
	if err != nil {
		return ComplianceMaterial{}, err
	}
	return ComplianceMaterial{Enabled: true, Mode: mode}, nil
}

func complianceFromForm(form *multipart.Form) (ComplianceMaterial, error) {
	values := map[string]any{}
	for _, key := range []string{"compliance_enabled", "eye_mask_enabled", "compliance_mode", "eye_mask_mode"} {
		if vs := form.Value[key]; len(vs) > 0 {
			values[key] = vs[0]
		}
	}
	if raw := strings.TrimSpace(firstFormValue(form, "metadata")); raw != "" {
		var meta map[string]any
		if common.Unmarshal([]byte(raw), &meta) == nil {
			values["metadata"] = meta
		}
	}
	return complianceFromMap(values)
}

func firstFormValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return form.Value[key][0]
}

func normalizeComplianceMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "fishnet":
		return "fishnet", nil
	case "grid":
		return "grid", nil
	case "watercolor":
		return "watercolor", nil
	case "colored-pencil", "colored_pencil", "pencil":
		return "colored-pencil", nil
	default:
		return "", fmt.Errorf("invalid compliance_mode %q", mode)
	}
}

func flexibleComplianceBool(v any) (enabled bool, present bool, err error) {
	if v == nil {
		return false, false, nil
	}
	switch t := v.(type) {
	case bool:
		return t, true, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "":
			return false, false, nil
		case "true", "1", "yes", "on":
			return true, true, nil
		case "false", "0", "no", "off":
			return false, true, nil
		default:
			return false, true, fmt.Errorf("invalid compliance_enabled")
		}
	case float64:
		if t == 1 {
			return true, true, nil
		}
		if t == 0 {
			return false, true, nil
		}
		return false, true, fmt.Errorf("invalid compliance_enabled")
	default:
		return false, true, fmt.Errorf("invalid compliance_enabled")
	}
}

func firstComplianceString(vals ...any) string {
	for _, v := range vals {
		if s := asComplianceString(v); s != "" {
			return s
		}
	}
	return ""
}

func collectImageValues(v any, inImage bool) []string {
	var out []string
	switch t := v.(type) {
	case map[string]any:
		if inImage {
			if s := mediaURL(t); s != "" {
				return []string{s}
			}
		}
		for k, child := range t {
			_, isImage := complianceImageKeys[k]
			out = append(out, collectImageValues(child, inImage || isImage)...)
		}
	case []any:
		for _, child := range t {
			out = append(out, collectImageValues(child, inImage)...)
		}
	case string:
		if inImage {
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func replaceImageValues(v any, inImage bool, repl map[string]string) any {
	switch t := v.(type) {
	case map[string]any:
		if inImage {
			if s := mediaURL(t); s != "" {
				if n, ok := repl[s]; ok {
					if _, has := t["url"]; has {
						t["url"] = n
						return t
					}
					if _, has := t["image_url"]; has {
						t["image_url"] = n
						return t
					}
				}
			}
		}
		for k, child := range t {
			_, isImage := complianceImageKeys[k]
			t[k] = replaceImageValues(child, inImage || isImage, repl)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = replaceImageValues(child, inImage, repl)
		}
		return t
	case string:
		if inImage {
			if n, ok := repl[strings.TrimSpace(t)]; ok {
				return n
			}
		}
		return t
	default:
		return v
	}
}

func mediaURL(m map[string]any) string {
	for _, key := range []string{"url", "image_url", "src"} {
		if s := asComplianceString(m[key]); strings.HasPrefix(s, "http") || strings.HasPrefix(s, "data:") {
			return s
		}
	}
	return ""
}

func stripComplianceFlags(m map[string]any) {
	if m == nil {
		return
	}
	for _, key := range []string{"compliance_enabled", "compliance_mode", "eye_mask_enabled", "eye_mask_mode"} {
		delete(m, key)
	}
	if meta, ok := m["metadata"].(map[string]any); ok {
		for _, key := range []string{"compliance_enabled", "compliance_mode", "eye_mask_enabled", "eye_mask_mode"} {
			delete(meta, key)
		}
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func complianceBodyIsJSON(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "application/json") || strings.Contains(ct, "+json") {
		return true
	}
	trim := bytes.TrimSpace(body)
	return len(trim) > 0 && trim[0] == '{'
}

func complianceReadForm(contentType string, body []byte) (*multipart.Form, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("multipart boundary missing")
	}
	return multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(64 << 20)
}

func complianceFormImages(form *multipart.Form) []string {
	if form == nil {
		return nil
	}
	var refs []string
	for key := range complianceImageKeys {
		refs = append(refs, form.Value[key]...)
	}
	for _, field := range []string{"image", "image[]", "images", "image_refs", "image_urls", "reference_images", "extra_images", "input_reference", "first_frame", "last_frame"} {
		for _, fh := range form.File[field] {
			refs = append(refs, "file:"+field+":"+fh.Filename)
		}
	}
	return uniqueStrings(refs)
}

func applyComplianceMultipart(ctx context.Context, body []byte, contentType string, opt ComplianceMaterial) ([]byte, string, error) {
	form, err := complianceReadForm(contentType, body)
	if err != nil {
		return nil, contentType, err
	}
	defer form.RemoveAll()
	refs := uniqueStrings(complianceFormImages(form))
	if len(refs) == 0 {
		return body, contentType, nil
	}
	sources := map[string]string{}
	var ordered []string
	for key := range complianceImageKeys {
		for _, v := range form.Value[key] {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := sources[v]; ok {
				continue
			}
			sources[v] = v
			ordered = append(ordered, v)
		}
	}
	fileSources := map[string]string{}
	for _, field := range []string{"image", "image[]", "images", "image_refs", "image_urls", "reference_images", "extra_images", "input_reference", "first_frame", "last_frame"} {
		for _, fh := range form.File[field] {
			id := "file:" + field + ":" + fh.Filename
			if _, ok := fileSources[id]; ok {
				continue
			}
			dataURL, err := complianceFileDataURL(fh)
			if err != nil {
				return nil, contentType, err
			}
			fileSources[id] = dataURL
			ordered = append(ordered, dataURL)
		}
	}
	replaced, edited, err := editComplianceImages(ctx, opt.Mode, uniqueStrings(ordered))
	if err != nil {
		return nil, contentType, err
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	wrotePrompt := false
	for key, vals := range form.Value {
		if _, drop := map[string]struct{}{"compliance_enabled": {}, "compliance_mode": {}, "eye_mask_enabled": {}, "eye_mask_mode": {}}[key]; drop {
			continue
		}
		for _, val := range vals {
			if src, ok := sources[strings.TrimSpace(val)]; ok {
				if n, ok := replaced[src]; ok {
					val = n
				}
			}
			if key == "prompt" {
				if edited > 0 {
					val = carryComplianceVideoPrompt(val, opt.Mode)
				}
				wrotePrompt = true
			}
			if err := writer.WriteField(key, val); err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
		}
	}
	if !wrotePrompt && edited > 0 {
		if err := writer.WriteField("prompt", carryComplianceVideoPrompt("", opt.Mode)); err != nil {
			_ = writer.Close()
			return nil, contentType, err
		}
	}
	for field, files := range form.File {
		for _, fh := range files {
			id := "file:" + field + ":" + fh.Filename
			data := ""
			if src, ok := fileSources[id]; ok {
				if rep := replaced[src]; rep != "" && rep != src {
					data = rep
				}
			}
			if data == "" {
				if err := complianceCopyFile(writer, field, fh); err != nil {
					_ = writer.Close()
					return nil, contentType, err
				}
				continue
			}
			raw, ct, err := decodeDataURL(data)
			if err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, fh.Filename))
			h.Set("Content-Type", ct)
			part, err := writer.CreatePart(h)
			if err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
			if _, err := part.Write(raw); err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, contentType, err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func complianceCopyFile(writer *multipart.Writer, field string, fh *multipart.FileHeader) error {
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	h := make(textproto.MIMEHeader)
	name := fh.Filename
	if name == "" {
		name = "file"
	}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, name))
	ct := fh.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	h.Set("Content-Type", ct)
	part, err := writer.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}

func complianceFileDataURL(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 32<<20))
	if err != nil {
		return "", err
	}
	ct := fh.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		ct = http.DetectContentType(data)
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func decodeDataURL(v string) ([]byte, string, error) {
	if !strings.HasPrefix(v, "data:") {
		return nil, "", fmt.Errorf("not a data url")
	}
	head, b64, ok := strings.Cut(v, ",")
	if !ok {
		return nil, "", fmt.Errorf("bad data url")
	}
	ct := strings.TrimPrefix(head, "data:")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	if ct == "" {
		ct = "image/png"
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(b64)
	}
	if err != nil {
		return nil, "", err
	}
	return raw, ct, nil
}
