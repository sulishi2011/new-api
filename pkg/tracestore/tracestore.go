package tracestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var ErrNotConfigured = errors.New("trace store is not configured")

type Store interface {
	Put(ctx context.Context, key string, body []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

var (
	mu          sync.RWMutex
	activeStore Store
	configured  bool
)

func Init() error {
	mu.Lock()
	defer mu.Unlock()

	activeStore = nil
	configured = false

	if !common.TraceStorageEnabled {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(common.TraceStorageBackend)) {
	case "", "s3":
		store, err := newS3Store(context.Background())
		if err != nil {
			return err
		}
		activeStore = store
		configured = true
		return nil
	default:
		return fmt.Errorf("unsupported trace storage backend: %s", common.TraceStorageBackend)
	}
}

func IsConfigured() bool {
	mu.RLock()
	defer mu.RUnlock()
	return configured && activeStore != nil
}

func Put(ctx context.Context, key string, body []byte) error {
	mu.RLock()
	store := activeStore
	ok := configured
	mu.RUnlock()
	if !ok || store == nil {
		return ErrNotConfigured
	}
	return store.Put(ctx, key, body)
}

func Get(ctx context.Context, key string) ([]byte, error) {
	mu.RLock()
	store := activeStore
	ok := configured
	mu.RUnlock()
	if !ok || store == nil {
		return nil, ErrNotConfigured
	}
	return store.Get(ctx, key)
}

type s3Store struct {
	client *s3.Client
	bucket string
	sse    string
}

func newS3Store(ctx context.Context) (*s3Store, error) {
	bucket := strings.TrimSpace(common.TraceS3Bucket)
	if bucket == "" {
		return nil, errors.New("TRACE_S3_BUCKET is required when TRACE_STORAGE_ENABLED=true")
	}
	region := strings.TrimSpace(common.TraceS3Region)
	if region == "" {
		region = "us-east-1"
	}

	loadOptions := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if common.TraceS3AccessKeyID != "" || common.TraceS3SecretAccessKey != "" {
		if common.TraceS3AccessKeyID == "" || common.TraceS3SecretAccessKey == "" {
			return nil, errors.New("TRACE_S3_ACCESS_KEY_ID and TRACE_S3_SECRET_ACCESS_KEY must be set together")
		}
		loadOptions = append(loadOptions, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			common.TraceS3AccessKeyID,
			common.TraceS3SecretAccessKey,
			"",
		)))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if common.TraceS3Endpoint != "" {
			o.BaseEndpoint = aws.String(common.TraceS3Endpoint)
		}
		o.UsePathStyle = common.TraceS3ForcePathStyle
	})
	return &s3Store{
		client: client,
		bucket: bucket,
		sse:    common.TraceS3SSE,
	}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, body []byte) error {
	input := &s3.PutObjectInput{
		Bucket:          aws.String(s.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(body),
		ContentType:     aws.String("application/json"),
		ContentEncoding: aws.String("gzip"),
	}
	if s.sse != "" {
		input.ServerSideEncryption = s3types.ServerSideEncryption(s.sse)
	}
	_, err := s.client.PutObject(ctx, input)
	return err
}

func (s *s3Store) Get(ctx context.Context, key string) ([]byte, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return io.ReadAll(output.Body)
}
