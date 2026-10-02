package r2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultAccountID     = "47488236385c8da653edce5e9f0b7f64"
	defaultBucket        = "domiex-videos"
	defaultPublicBaseURL = "https://oss.domie.studio"
	defaultRegion        = "auto"
)

var ErrDisabled = errors.New("r2 is not configured")

type Config struct {
	AccountID       string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
}

func LoadConfig() Config {
	accountID := firstNonEmpty(os.Getenv("R2_ACCOUNT_ID"), defaultAccountID)
	return Config{
		AccountID:       accountID,
		Endpoint:        firstNonEmpty(os.Getenv("R2_ENDPOINT"), r2Endpoint(accountID)),
		Region:          firstNonEmpty(os.Getenv("R2_REGION"), defaultRegion),
		Bucket:          firstNonEmpty(os.Getenv("R2_BUCKET"), defaultBucket),
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		PublicBaseURL:   strings.TrimRight(firstNonEmpty(os.Getenv("R2_PUBLIC_BASE_URL"), defaultPublicBaseURL), "/"),
	}
}

func (c Config) Enabled() bool {
	return strings.TrimSpace(c.Bucket) != "" &&
		strings.TrimSpace(c.AccessKeyID) != "" &&
		strings.TrimSpace(c.SecretAccessKey) != "" &&
		strings.TrimSpace(c.PublicBaseURL) != "" &&
		strings.TrimSpace(c.Endpoint) != ""
}

func (c Config) PublicURL(key string) string {
	base := strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if base == "" || key == "" {
		return ""
	}
	return base + "/" + key
}

func (c Config) IsPublicURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	base := strings.TrimSpace(c.PublicBaseURL)
	if raw == "" || base == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	baseParsed, err := url.Parse(base)
	if err != nil || baseParsed.Host == "" {
		return false
	}
	if !strings.EqualFold(parsed.Host, baseParsed.Host) {
		return false
	}
	return strings.HasPrefix(parsed.EscapedPath(), "/videos/")
}

func IsPublicObjectURL(raw string) bool {
	return LoadConfig().IsPublicURL(raw)
}

func PublicObjectURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !IsPublicObjectURL(raw) {
		return ""
	}
	return raw
}

func ObjectKey(taskID string, at time.Time) (string, error) {
	id := strings.TrimSpace(taskID)
	if id == "" || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return "", errors.New("invalid task id")
	}
	if !strings.HasPrefix(id, "task_") {
		id = "task_" + id
	}
	if at.IsZero() {
		at = time.Now()
	}
	return fmt.Sprintf("videos/%s/%s.mp4", at.UTC().Format("2006/01"), id), nil
}

type Client struct {
	cfg Config
	s3  *s3.Client
}

func NewClient(cfg Config) (*Client, error) {
	if !cfg.Enabled() {
		return nil, ErrDisabled
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = defaultRegion
	}
	client := s3.New(s3.Options{
		Region:                     region,
		Credentials:                aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		BaseEndpoint:               aws.String(strings.TrimSpace(cfg.Endpoint)),
		UsePathStyle:               true,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &Client{cfg: cfg, s3: client}, nil
}

func (c *Client) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) (string, error) {
	if c == nil || c.s3 == nil {
		return "", ErrDisabled
	}
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if key == "" {
		return "", errors.New("empty object key")
	}
	if contentType == "" {
		contentType = "video/mp4"
	}
	input := &s3.PutObjectInput{
		Bucket:       aws.String(c.cfg.Bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}
	if _, err := c.s3.PutObject(ctx, input); err != nil {
		return "", err
	}
	return c.cfg.PublicURL(key), nil
}

func r2Endpoint(accountID string) string {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return ""
	}
	return "https://" + accountID + ".r2.cloudflarestorage.com"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
