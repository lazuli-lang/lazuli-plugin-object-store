// provider.go — `storage.Provider` implementation. Provider is the
// vendor-neutral object-store contract the Lazuli runtime exposes via
// `lazuli.ObjectStore("<binding>")` (spec:
// docs/proposals/hostpoint-complete-roadmap-2026-05-18.md §3.5).
// The legacy `storage.ObjectStore` adapter shape (above) stays for
// the in-tree storage helpers; this file wires the runtime-facing
// contract atop the same SDK client.
//
// The adapter ignores the `bucket` arg on calls when its configured
// `Cfg.Bucket` is non-empty (single-bucket per process is the
// historical shape). When `Cfg.Bucket` is empty the `bucket` arg
// passes straight through to the SDK call, so multi-bucket pilots
// can drive S3 via per-call routing.

package object_store

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"lazuli.dev/runtime/lazuli/storage"
)

// Compile-time check: the adapter satisfies `storage.Provider`. When
// `storage.Provider` grows additive methods, the build fails here
// with a clear "missing method" diagnostic.
var _ storage.Provider = (*Adapter)(nil)

// resolveBucket returns the bucket argument the adapter routes to.
// Per-process Cfg.Bucket wins so legacy single-bucket pilots keep
// working; otherwise the call-site `bucket` is honoured.
func (a *Adapter) resolveBucket(bucket string) string {
	if a.cfg.Bucket != "" {
		return a.cfg.Bucket
	}
	return bucket
}

// PutObject — `storage.Provider`.
func (a *Adapter) PutObject(ctx context.Context, bucket, key, contentType string, body io.Reader) error {
	if a.err != nil {
		return a.err
	}
	_, err := a.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.resolveBucket(bucket)),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return mapErr(err)
}

// GetObject — `storage.Provider`.
func (a *Adapter) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, string, error) {
	if a.err != nil {
		return nil, "", a.err
	}
	out, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.resolveBucket(bucket)),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", mapErr(err)
	}
	mime := ""
	if out.ContentType != nil {
		mime = *out.ContentType
	}
	return out.Body, mime, nil
}

// DeleteObject — `storage.Provider`. HEAD-then-DELETE so a missing
// object surfaces as `storage.ErrFileNotFound` (S3 DeleteObject is
// idempotent and returns 204 even on a miss).
func (a *Adapter) DeleteObject(ctx context.Context, bucket, key string) error {
	if a.err != nil {
		return a.err
	}
	target := a.resolveBucket(bucket)
	if _, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(target),
		Key:    aws.String(key),
	}); err != nil {
		return mapErr(err)
	}
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(target),
		Key:    aws.String(key),
	})
	return mapErr(err)
}

// ListPrefix — `storage.Provider`. Streams keys page-by-page through
// the iterator so callers don't materialise the full listing in
// memory before iterating.
func (a *Adapter) ListPrefix(ctx context.Context, bucket, prefix string) iter.Seq2[storage.ObjectMeta, error] {
	return func(yield func(storage.ObjectMeta, error) bool) {
		if a.err != nil {
			yield(storage.ObjectMeta{}, a.err)
			return
		}
		pager := s3.NewListObjectsV2Paginator(a.client, &s3.ListObjectsV2Input{
			Bucket: aws.String(a.resolveBucket(bucket)),
			Prefix: aws.String(prefix),
		})
		for pager.HasMorePages() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				yield(storage.ObjectMeta{}, mapErr(err))
				return
			}
			for _, obj := range page.Contents {
				meta := storage.ObjectMeta{}
				if obj.Key != nil {
					meta.Key = *obj.Key
				}
				if obj.Size != nil {
					meta.Size = *obj.Size
				}
				if obj.LastModified != nil {
					meta.LastModified = *obj.LastModified
				}
				if !yield(meta, nil) {
					return
				}
			}
		}
	}
}

// PresignedURL — `storage.Provider`. Closed catalog on `method`:
// "GET" (download) or "PUT" (upload). The runtime never mints DELETE
// or POST URLs.
func (a *Adapter) PresignedURL(ctx context.Context, bucket, key string, ttl time.Duration, method string) (string, error) {
	if ttl <= 0 {
		return "", storage.ErrVisibilityMismatch
	}
	if a.err != nil {
		return "", a.err
	}
	target := a.resolveBucket(bucket)
	switch method {
	case "GET":
		req, err := a.presign.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(target),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(ttl))
		if err != nil {
			return "", mapErr(err)
		}
		return req.URL, nil
	case "PUT":
		req, err := a.presign.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(target),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(ttl))
		if err != nil {
			return "", mapErr(err)
		}
		return req.URL, nil
	default:
		return "", errors.New("@plugin/object-store: PresignedURL method must be \"GET\" or \"PUT\"")
	}
}

// Compile-time guard so a future provider type-rename surfaces here.
var _ = types.NoSuchKey{}
