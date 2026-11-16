package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/r2"
	"golang.org/x/sync/singleflight"
)

// Provider file links stop working after a few hours. Keep trying to copy
// them until shortly before that cutoff.
const providerVideoMirrorWindow = 170 * time.Minute

var errProviderVideoExpired = errors.New("provider video expired")
var errProviderVideoBusy = errors.New("provider video pull is busy")
var errProviderVideoCooling = errors.New("provider video pull cooling down")

const providerVideoPullCooldown = 20 * time.Second

var (
	providerVideoMirrorJobs  sync.Map
	providerVideoMirrorStop  sync.Map
	providerVideoPullAfter   sync.Map
	providerVideoStoreFlight singleflight.Group
	providerVideoMirrorSlots = make(chan struct{}, 2)
)

var saveMirroredVideoResult = func(task *model.Task, publicURL string) error {
	task.PrivateData.ResultURL = publicURL
	task.PrivateData.DirectUpstreamVideo = false
	return task.UpdatePrivateData()
}

// ScheduleProviderVideoMirror copies a finished provider file into object
// storage without blocking the caller. A second call for the same task waits
// on the in-flight copy instead of starting another download.
func ScheduleProviderVideoMirror(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || providerVideoMirrorStopped(taskID) || !r2.LoadConfig().Enabled() {
		return false
	}
	if _, loaded := providerVideoMirrorJobs.LoadOrStore(taskID, struct{}{}); loaded {
		return false
	}
	go func() {
		defer providerVideoMirrorJobs.Delete(taskID)
		ctx, cancel := context.WithTimeout(context.Background(), providerVideoDownloadTimeout+time.Minute)
		defer cancel()
		publicURL, err := storeProviderVideoByID(taskID)
		if err != nil {
			if errors.Is(err, errProviderVideoExpired) || providerVideoMirrorStopped(taskID) {
				providerVideoMirrorStop.Store(taskID, struct{}{})
				dropProviderVideoMirror(taskID)
				logger.LogWarn(ctx, fmt.Sprintf("provider video expired before it could be stored: %s", taskID))
				return
			}
			if errors.Is(err, errProviderVideoCooling) {
				return
			}
			rememberProviderVideoMirror(taskID)
			if !errors.Is(err, errProviderVideoBusy) {
				logger.LogWarn(ctx, fmt.Sprintf("provider video mirror failed for %s: %v", taskID, err))
			}
			return
		}
		if publicURL == "" {
			dropProviderVideoMirror(taskID)
			return
		}
		dropProviderVideoMirror(taskID)
	}()
	return true
}

func providerVideoMirrorStopped(taskID string) bool {
	_, ok := providerVideoMirrorStop.Load(taskID)
	return ok
}

var (
	mirrorBacklogMu   sync.Mutex
	mirrorBacklog     []string
	mirrorBacklogLoad sync.Once
)

func mirrorRecentProviderVideos(ctx context.Context) {
	if !r2.LoadConfig().Enabled() {
		return
	}
	mirrorBacklogLoad.Do(func() {
		go loadProviderVideoMirrorBacklog(ctx)
	})
	if len(providerVideoMirrorSlots) >= cap(providerVideoMirrorSlots) {
		return
	}
	mirrorBacklogMu.Lock()
	ids := append([]string(nil), mirrorBacklog...)
	mirrorBacklogMu.Unlock()
	scheduled := 0
	room := cap(providerVideoMirrorSlots) - len(providerVideoMirrorSlots)
	for _, taskID := range ids {
		if providerVideoMirrorStopped(taskID) {
			dropProviderVideoMirror(taskID)
			continue
		}
		if !ScheduleProviderVideoMirror(taskID) {
			continue
		}
		scheduled++
		if scheduled >= room {
			break
		}
	}
	if scheduled > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("scheduled %d provider video copies", scheduled))
	}
}

func loadProviderVideoMirrorBacklog(ctx context.Context) {
	since := time.Now().Add(-providerVideoMirrorWindow).Unix()
	tasks := model.GetRecentUnstoredSuccessfulTasks(since, 1000)
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task != nil && providerVideoNeedsMirror(nil, task) {
			ids = append(ids, task.TaskID)
		}
	}
	mirrorBacklogMu.Lock()
	seen := make(map[string]struct{}, len(ids)+len(mirrorBacklog))
	merged := make([]string, 0, len(ids)+len(mirrorBacklog))
	for _, id := range append(ids, mirrorBacklog...) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	mirrorBacklog = merged
	mirrorBacklogMu.Unlock()
	logger.LogInfo(ctx, fmt.Sprintf("provider video copy backlog: %d", len(ids)))
}

func rememberProviderVideoMirror(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	mirrorBacklogMu.Lock()
	defer mirrorBacklogMu.Unlock()
	for _, id := range mirrorBacklog {
		if id == taskID {
			return
		}
	}
	mirrorBacklog = append(mirrorBacklog, taskID)
}

func dropProviderVideoMirror(taskID string) {
	mirrorBacklogMu.Lock()
	defer mirrorBacklogMu.Unlock()
	kept := mirrorBacklog[:0]
	for _, id := range mirrorBacklog {
		if id != taskID {
			kept = append(kept, id)
		}
	}
	mirrorBacklog = kept
}

// StoreProviderVideoOnce copies a provider file that was left behind the
// gateway content URL. Concurrent callers share one download. A stored object
// URL is returned for the caller to redirect to.
func StoreProviderVideoOnce(ctx context.Context, task *model.Task, ch *model.Channel) string {
	if task == nil || ch == nil || providerVideoMirrorStopped(task.TaskID) || !r2.LoadConfig().Enabled() {
		return ""
	}
	if publicURL := r2.PublicObjectURL(task.PrivateData.ResultURL); publicURL != "" {
		return publicURL
	}
	if !providerVideoNeedsMirror(ch, task) || providerVideoCoolingDown(task.TaskID) {
		return ""
	}
	type outcome struct {
		url string
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		url, err := storeProviderVideo(task, ch)
		done <- outcome{url: url, err: err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			if !errors.Is(res.err, errProviderVideoCooling) && !errors.Is(res.err, errProviderVideoBusy) {
				logger.LogWarn(ctx, fmt.Sprintf("provider video pull failed for %s: %v", task.TaskID, res.err))
			}
			return ""
		}
		return res.url
	case <-ctx.Done():
		return ""
	}
}

func storeProviderVideoByID(taskID string) (string, error) {
	task, exists, err := model.GetByOnlyTaskId(taskID)
	if err != nil || !exists || task == nil {
		return "", err
	}
	ch, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		ch, err = model.GetChannelById(task.ChannelId, true)
		if err != nil {
			return "", err
		}
	}
	return storeProviderVideo(task, ch)
}

func storeProviderVideo(task *model.Task, ch *model.Channel) (string, error) {
	if task == nil || strings.TrimSpace(task.TaskID) == "" {
		return "", nil
	}
	if publicURL := r2.PublicObjectURL(task.PrivateData.ResultURL); publicURL != "" {
		return publicURL, nil
	}
	if providerVideoMirrorStopped(task.TaskID) {
		return "", nil
	}
	if providerVideoCoolingDown(task.TaskID) {
		return "", errProviderVideoCooling
	}
	v, err, _ := providerVideoStoreFlight.Do(task.TaskID, func() (any, error) {
		pullCtx, cancel := context.WithTimeout(context.Background(), providerVideoDownloadTimeout)
		defer cancel()
		select {
		case providerVideoMirrorSlots <- struct{}{}:
			defer func() { <-providerVideoMirrorSlots }()
		case <-time.After(30 * time.Second):
			return "", errProviderVideoBusy
		case <-pullCtx.Done():
			return "", errProviderVideoBusy
		}
		publicURL, err := mirrorProviderVideoTask(pullCtx, task, ch)
		if err != nil {
			if errors.Is(err, errProviderVideoExpired) {
				providerVideoMirrorStop.Store(task.TaskID, struct{}{})
			} else if !errors.Is(err, errProviderVideoBusy) {
				providerVideoPullAfter.Store(task.TaskID, time.Now().Add(providerVideoPullCooldown))
			}
			return "", err
		}
		providerVideoPullAfter.Delete(task.TaskID)
		return publicURL, nil
	})
	if err != nil {
		return "", err
	}
	publicURL, _ := v.(string)
	if publicURL != "" {
		task.PrivateData.ResultURL = publicURL
		task.PrivateData.DirectUpstreamVideo = false
	}
	return publicURL, nil
}

func providerVideoCoolingDown(taskID string) bool {
	value, ok := providerVideoPullAfter.Load(taskID)
	if !ok {
		return false
	}
	until, ok := value.(time.Time)
	if !ok || time.Now().After(until) {
		providerVideoPullAfter.Delete(taskID)
		return false
	}
	return true
}

func providerVideoNeedsMirror(ch *model.Channel, task *model.Task) bool {
	if task == nil || task.Status != model.TaskStatusSuccess || task.PrivateData.DirectUpstreamVideo {
		return false
	}
	if task.Platform == constant.TaskPlatformImage || task.Platform == constant.TaskPlatformPay {
		return false
	}
	if r2.PublicObjectURL(task.PrivateData.ResultURL) != "" {
		return false
	}
	raw := strings.TrimSpace(task.PrivateData.UpstreamResultURL)
	if raw == "" {
		return false
	}
	if ch == nil {
		return !model.ClientMayReceiveDirectVideoURL(raw)
	}
	return !clientMayKeepUpstreamVideoURL(ch, absoluteUpstreamVideoURL(ch, raw))
}

func mirrorProviderVideoTask(ctx context.Context, task *model.Task, ch *model.Channel) (string, error) {
	if !providerVideoNeedsMirror(ch, task) {
		return r2.PublicObjectURL(task.PrivateData.ResultURL), nil
	}
	if ch == nil {
		return "", errors.New("channel is missing")
	}
	body, err := downloadProviderVideo(ctx, ch, task)
	if err != nil {
		return "", err
	}
	publicURL := persistGeneratedVideoToR2(ctx, task, nil, body)
	if publicURL == "" {
		return "", errors.New("object storage upload returned no url")
	}
	if err := saveMirroredVideoResult(task, publicURL); err != nil {
		return "", err
	}
	logger.LogInfo(ctx, fmt.Sprintf("stored provider video %s at %s", task.TaskID, publicURL))
	return publicURL, nil
}

func downloadProviderVideo(ctx context.Context, ch *model.Channel, task *model.Task) ([]byte, error) {
	client, err := providerVideoHTTPClient(ch.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(task.PrivateData.Key)
	if apiKey == "" {
		apiKey = strings.TrimSpace(ch.Key)
	}
	baseURL := ch.GetBaseURL()
	upstreamID := task.GetUpstreamTaskID()
	fresh := refreshProviderVideoURL(ctx, client, baseURL, apiKey, upstreamID)
	stored := absoluteUpstreamVideoURL(ch, task.PrivateData.UpstreamResultURL)
	contentURL := providerTaskContentURL(baseURL, upstreamID)
	candidates := make([]string, 0, 3)
	for _, candidate := range []string{fresh, stored, contentURL} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		seen := false
		for _, existing := range candidates {
			if existing == candidate {
				seen = true
				break
			}
		}
		if !seen {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("provider video url is empty")
	}
	var last error
	expiredOnly := true
	for _, candidate := range candidates {
		body, err := downloadOneProviderVideo(ctx, client, baseURL, apiKey, candidate)
		if err == nil {
			return body, nil
		}
		last = err
		if !errors.Is(err, errProviderVideoExpired) {
			expiredOnly = false
		}
	}
	if expiredOnly {
		return nil, errProviderVideoExpired
	}
	if last == nil || errors.Is(last, errProviderVideoExpired) {
		return nil, errors.New("provider video download failed")
	}
	return nil, last
}

func providerTaskContentURL(baseURL, upstreamTaskID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if baseURL == "" || upstreamTaskID == "" {
		return ""
	}
	return baseURL + "/v1/videos/" + url.PathEscape(upstreamTaskID) + "/content"
}

// NewProviderVideoHTTPClient downloads provider files on a fresh HTTP/1.1
// connection. The shared relay client speaks HTTP/2, and a stuck HTTP/2
// session makes content links hang until the request deadline.
func NewProviderVideoHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   20 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
		ResponseHeaderTimeout: 45 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	if strings.TrimSpace(proxyURL) != "" {
		base, err := GetHttpClientWithProxy(proxyURL)
		if err != nil {
			return nil, err
		}
		if base != nil {
			if proxied, ok := base.Transport.(*http.Transport); ok && proxied != nil && proxied.Proxy != nil {
				transport.Proxy = proxied.Proxy
			}
			if proxied, ok := base.Transport.(*http.Transport); ok && proxied != nil && proxied.DialContext != nil {
				transport.DialContext = proxied.DialContext
			}
		}
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       providerVideoDownloadTimeout,
		CheckRedirect: checkRedirect,
	}, nil
}

func providerVideoHTTPClient(proxyURL string) (*http.Client, error) {
	return NewProviderVideoHTTPClient(proxyURL)
}

func refreshProviderVideoURL(ctx context.Context, client *http.Client, baseURL, apiKey, upstreamTaskID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if baseURL == "" || upstreamTaskID == "" || client == nil {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/videos/"+url.PathEscape(upstreamTaskID), nil)
	if err != nil {
		return ""
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(extractCompletedVideoURL(body))
}

func downloadOneProviderVideo(ctx context.Context, client *http.Client, baseURL, apiKey, videoURL string) ([]byte, error) {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return nil, errors.New("empty video url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, err
	}
	if videoURLNeedsChannelAuth(videoURL) && apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		return nil, errProviderVideoExpired
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("provider video HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxVideoValidationBytes {
		return nil, fmt.Errorf("provider video exceeds %d bytes", maxVideoValidationBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVideoValidationBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxVideoValidationBytes {
		return nil, fmt.Errorf("provider video exceeds %d bytes", maxVideoValidationBytes)
	}
	if !providerVideoBodyOK(resp.Header.Get("Content-Type"), body) {
		return nil, errors.New("provider response is not a video")
	}
	return body, nil
}

func providerVideoBodyOK(contentType string, body []byte) bool {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "text/xml") ||
		strings.Contains(contentType, "application/xml") {
		return false
	}
	if len(body) < 12 {
		return false
	}
	head := body
	if len(head) > 64 {
		head = head[:64]
	}
	return bytes.Contains(head, []byte("ftyp"))
}
