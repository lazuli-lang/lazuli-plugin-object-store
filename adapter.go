// Package object_store is the Lazuli @plugin/object-store adapter.
// Implements lazuli.dev/runtime/lazuli/storage.ObjectStore by wrapping
// aws-sdk-go-v2/service/s3. One wire, many providers: AWS S3, MinIO,
// Cloudflare R2, Tigris, and Backblaze B2 diverge only by endpoint
// URL, path-style preference, and region literal — all env-driven
// (see Config). Wire-thin: each method is a single SDK call. Retry
// and circuit-breaker concerns belong to the Lazuli runtime.
package object_store

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"lazuli.dev/runtime/lazuli"
	"lazuli.dev/runtime/lazuli/storage"
)

const AdapterRef = "@plugin/object-store"

// Config is single-bucket per process. Multi-bucket products
// instantiate one Adapter per bucket and register under distinct
// AdapterRef values.
type Config struct {
	Endpoint       string
	Region         string
	AccessKeyID    string
	SecretKey      string
	ForcePathStyle bool
	Bucket         string
}

type Adapter struct {
	client  *s3.Client
	presign *s3.PresignClient
	cfg     Config
	err     error
}

var _ storage.ObjectStore = (*Adapter)(nil)

func init() {
	lazuli.RegisterAdapter(AdapterRef, newAdapter())
}

func newAdapter() *Adapter {
	cfg := loadConfig()
	client, err := buildClient(context.Background(), cfg)
	if err != nil {
		return &Adapter{cfg: cfg, err: err}
	}
	return &Adapter{
		client:  client,
		presign: s3.NewPresignClient(client),
		cfg:     cfg,
	}
}

func loadConfig() Config {
	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	return Config{
		Endpoint:       os.Getenv("S3_ENDPOINT"),
		Region:         region,
		AccessKeyID:    os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:      os.Getenv("S3_SECRET_ACCESS_KEY"),
		ForcePathStyle: strings.EqualFold(os.Getenv("S3_FORCE_PATH_STYLE"), "true"),
		Bucket:         os.Getenv("S3_BUCKET_DEFAULT"),
	}
}

func buildClient(ctx context.Context, c Config) (*s3.Client, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(c.Region)}
	if c.AccessKeyID != "" && c.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretKey, ""),
		))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
		o.UsePathStyle = c.ForcePathStyle
	}), nil
}

func (a *Adapter) Put(ctx context.Context, key storage.Key, body io.Reader, contentType string) error {
	if a.err != nil {
		return a.err
	}
	_, err := a.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.cfg.Bucket),
		Key:         aws.String(string(key)),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return mapErr(err)
}

func (a *Adapter) Get(ctx context.Context, key storage.Key) (io.ReadCloser, error) {
	if a.err != nil {
		return nil, a.err
	}
	out, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return out.Body, nil
}

func (a *Adapter) Delete(ctx context.Context, key storage.Key) error {
	if a.err != nil {
		return a.err
	}
	// HEAD first so a missing object surfaces as storage.ErrFileNotFound
	// (S3 DeleteObject is idempotent and returns 204 even on a miss).
	if _, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(string(key)),
	}); err != nil {
		return mapErr(err)
	}
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(string(key)),
	})
	return mapErr(err)
}

func (a *Adapter) Sign(ctx context.Context, key storage.Key, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", storage.ErrVisibilityMismatch
	}
	if a.err != nil {
		return "", a.err
	}
	req, err := a.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.cfg.Bucket),
		Key:    aws.String(string(key)),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", mapErr(err)
	}
	return req.URL, nil
}

// ListPrefix lives in provider.go (storage.Provider implementation).
// The legacy single-arg shape (`func (a *Adapter) ListPrefix(ctx, prefix)`)
// was retired alongside the storage.Provider contract landing per
// docs/proposals/hostpoint-complete-roadmap-2026-05-18.md §3.5. The
// Firebase Storage migrator now consumes the iter.Seq2 form.

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return storage.ErrFileNotFound
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return storage.ErrFileNotFound
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchBucket":
			return storage.ErrFileNotFound
		case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return ErrUnauthorized
		}
	}
	return err
}
