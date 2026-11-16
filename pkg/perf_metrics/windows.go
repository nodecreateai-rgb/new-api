package perfmetrics

import (
	"math"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const (
	successWindow24h int64 = 24 * 3600
	successWindow12h int64 = 12 * 3600
	successWindow3h  int64 = 3 * 3600
)

type ModelSuccessWindows struct {
	ModelName        string   `json:"model_name"`
	SuccessRate24h   *float64 `json:"success_rate_24h"`
	SuccessRate12h   *float64 `json:"success_rate_12h"`
	SuccessRate3h    *float64 `json:"success_rate_3h"`
	AvgGenerationSec *int64   `json:"avg_generation_sec"`
}

type SuccessWindowsResult struct {
	Models []ModelSuccessWindows `json:"models"`
}

type windowCounts struct {
	requests int64
	success  int64
}

type successWindows struct {
	h24 windowCounts
	h12 windowCounts
	h3  windowCounts
}

func (w *windowCounts) add(requests int64, success int64) {
	w.requests += requests
	w.success += success
}

// add counts a bucket when its start falls inside each window. Bucket
// membership matches QuerySummaryAll: bucket_ts >= now-window and <= now.
func (w *successWindows) add(now int64, bucketTs int64, requests int64, success int64) {
	if requests <= 0 || bucketTs > now {
		return
	}
	if bucketTs >= now-successWindow24h {
		w.h24.add(requests, success)
	}
	if bucketTs >= now-successWindow12h {
		w.h12.add(requests, success)
	}
	if bucketTs >= now-successWindow3h {
		w.h3.add(requests, success)
	}
}

func (w *successWindows) summary(name string) ModelSuccessWindows {
	return ModelSuccessWindows{
		ModelName:      name,
		SuccessRate24h: roundedSuccessRate(w.h24),
		SuccessRate12h: roundedSuccessRate(w.h12),
		SuccessRate3h:  roundedSuccessRate(w.h3),
	}
}

func roundedSuccessRate(value windowCounts) *float64 {
	if value.requests <= 0 {
		return nil
	}
	rate := float64(value.success) / float64(value.requests) * 100
	rounded := math.Round(rate*100) / 100
	return &rounded
}

func rateFromCounts(success int64, failure int64) *float64 {
	total := success + failure
	if total <= 0 {
		return nil
	}
	if success < 0 {
		success = 0
	}
	rate := float64(success) / float64(total) * 100
	rounded := math.Round(rate*100) / 100
	return &rounded
}

func avgGenerationSec(sum int64, count int64) *int64 {
	if count <= 0 || sum <= 0 {
		return nil
	}
	seconds := sum / count
	if seconds <= 0 {
		return nil
	}
	return &seconds
}

func QuerySuccessWindows(groups []string) (SuccessWindowsResult, error) {
	now := time.Now().Unix()
	byName := map[string]ModelSuccessWindows{}

	taskRows, err := model.GetTaskSuccessWindows(now)
	if err != nil {
		return SuccessWindowsResult{}, err
	}
	for _, row := range taskRows {
		if row.ModelName == "" {
			continue
		}
		item := ModelSuccessWindows{
			ModelName:        row.ModelName,
			SuccessRate24h:   rateFromCounts(row.Success24h, row.Failure24h),
			SuccessRate12h:   rateFromCounts(row.Success12h, row.Failure12h),
			SuccessRate3h:    rateFromCounts(row.Success3h, row.Failure3h),
			AvgGenerationSec: avgGenerationSec(row.DurationSum24h, row.DurationCount24h),
		}
		if item.SuccessRate24h == nil && item.SuccessRate12h == nil && item.SuccessRate3h == nil {
			continue
		}
		byName[row.ModelName] = item
	}

	perfResult, err := queryPerfSuccessWindows(now, groups)
	if err != nil {
		return SuccessWindowsResult{}, err
	}
	for _, item := range perfResult.Models {
		if _, exists := byName[item.ModelName]; exists {
			continue
		}
		byName[item.ModelName] = item
	}

	models := make([]ModelSuccessWindows, 0, len(byName))
	for _, item := range byName {
		models = append(models, item)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ModelName < models[j].ModelName
	})
	return SuccessWindowsResult{Models: models}, nil
}

func queryPerfSuccessWindows(now int64, groups []string) (SuccessWindowsResult, error) {
	startTs := now - successWindow24h
	allowedGroups := allowedGroupSet(groups)

	rows, err := model.GetPerfMetricBucketCounts(startTs, now, groups)
	if err != nil {
		return SuccessWindowsResult{}, err
	}

	totals := map[string]*successWindows{}
	ensure := func(name string) *successWindows {
		window := totals[name]
		if window == nil {
			window = &successWindows{}
			totals[name] = window
		}
		return window
	}

	for _, row := range rows {
		ensure(row.ModelName).add(now, row.BucketTs, row.RequestCount, row.SuccessCount)
	}

	hotBuckets.Range(func(key, value any) bool {
		bucketKey := key.(bucketKey)
		if bucketKey.bucketTs < startTs || bucketKey.bucketTs > now {
			return true
		}
		if allowedGroups != nil {
			if _, ok := allowedGroups[bucketKey.group]; !ok {
				return true
			}
		}
		snap := value.(*atomicBucket).snapshot()
		ensure(bucketKey.model).add(now, bucketKey.bucketTs, snap.requestCount, snap.successCount)
		return true
	})

	models := make([]ModelSuccessWindows, 0, len(totals))
	for name, window := range totals {
		summary := window.summary(name)
		if summary.SuccessRate24h == nil && summary.SuccessRate12h == nil && summary.SuccessRate3h == nil {
			continue
		}
		models = append(models, summary)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ModelName < models[j].ModelName
	})

	return SuccessWindowsResult{Models: models}, nil
}
