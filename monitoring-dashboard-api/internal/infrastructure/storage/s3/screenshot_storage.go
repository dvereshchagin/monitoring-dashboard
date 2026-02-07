package s3

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type URLMode string

const (
	URLModePresigned URLMode = "presigned"
	URLModePublic    URLMode = "public"
)

type Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	URLMode         URLMode
	PresignedTTL    time.Duration
}

type ScreenshotStorage struct {
	client       *s3.Client
	presign      *s3.PresignClient
	bucket       string
	endpoint     string
	usePathStyle bool
	urlMode      URLMode
	presignedTTL time.Duration
}

func NewScreenshotStorage(ctx context.Context, cfg Config) (*ScreenshotStorage, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, fmt.Errorf("s3 access key id and secret are required")
	}
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = "ru-central1"
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = "https://storage.yandexcloud.net"
	}
	if cfg.URLMode == "" {
		cfg.URLMode = URLModePresigned
	}
	if cfg.URLMode != URLModePresigned && cfg.URLMode != URLModePublic {
		return nil, fmt.Errorf("unsupported s3 url mode: %s", cfg.URLMode)
	}
	if cfg.PresignedTTL <= 0 {
		cfg.PresignedTTL = 5 * time.Minute
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = &cfg.Endpoint
		options.UsePathStyle = cfg.UsePathStyle
	})

	return &ScreenshotStorage{
		client:       client,
		presign:      s3.NewPresignClient(client),
		bucket:       strings.TrimSpace(cfg.Bucket),
		endpoint:     strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"),
		usePathStyle: cfg.UsePathStyle,
		urlMode:      cfg.URLMode,
		presignedTTL: cfg.PresignedTTL,
	}, nil
}

func (s *ScreenshotStorage) PutObject(ctx context.Context, key, contentType string, body []byte) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("object key is required")
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        bytes.NewReader(body),
		ContentType: &contentType,
	})
	if err != nil {
		return "", fmt.Errorf("put object failed: %w", err)
	}

	if s.urlMode == URLModePublic {
		return s.publicURL(key), nil
	}

	request, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(s.presignedTTL))
	if err != nil {
		return "", fmt.Errorf("presign failed: %w", err)
	}

	return request.URL, nil
}

func (s *ScreenshotStorage) publicURL(key string) string {
	escapedKey := url.PathEscape(key)
	escapedKey = strings.ReplaceAll(escapedKey, "%2F", "/")
	if s.usePathStyle {
		return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, escapedKey)
	}
	endpoint := strings.TrimPrefix(s.endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return fmt.Sprintf("https://%s.%s/%s", s.bucket, endpoint, escapedKey)
}
