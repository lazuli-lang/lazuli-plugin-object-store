# lazuli-plugin-object-store

Lazuli `@plugin/object-store` adapter. Single S3-protocol wire that
works against AWS S3, MinIO, Cloudflare R2, Tigris, and Backblaze B2.

## Status

- ✅ **Go server adapter** — wraps `aws-sdk-go-v2/service/s3` for
  `Put` / `Get` / `Delete` / `Sign` (presigned GET) / `ListPrefix`.
  Implements `lazuli.dev/runtime/lazuli/storage.ObjectStore`.
  Auto-registers via `init()` against `@plugin/object-store`.
- ⏸ **`storage.Provider` migration** — when §3.5 of
  `hostpoint-complete-roadmap-2026-05-18.md` lands the runtime
  contract grows multi-bucket + `ObjectMeta` shape; this adapter
  rebinds against it without an API rewrite.
- ❌ **TS web / mobile** — not applicable. Object stores are
  server-only; signed URLs are the browser/mobile surface.

## Usage

In `Lazurite.toml`:

```toml
[plugins]
"@plugin/object-store" = { module = "lazuli.dev/plugin/object-store", version = "v0.1.0" }
```

In `registry.lzi`:

```lzi
registry
  bindings
    object_store: ObjectStore
      adapter @plugin/object-store
```

### Environment

| Variable | Required | Default | Notes |
|---|---|---|---|
| `S3_BUCKET_DEFAULT` | yes | — | bucket the adapter operates on |
| `S3_ENDPOINT` | no | AWS default | set to `http://localhost:9000` for MinIO |
| `S3_REGION` | no | `us-east-1` | MinIO accepts any value |
| `S3_ACCESS_KEY_ID` | no | — | omit when running with IAM role |
| `S3_SECRET_ACCESS_KEY` | no | — | omit when running with IAM role |
| `S3_FORCE_PATH_STYLE` | no | `false` | set `true` for MinIO |

## Local development

```bash
docker compose up -d
export S3_ENDPOINT=http://localhost:9000
export S3_REGION=us-east-1
export S3_ACCESS_KEY_ID=minioadmin
export S3_SECRET_ACCESS_KEY=minioadmin
export S3_FORCE_PATH_STYLE=true
export S3_BUCKET_DEFAULT=hostpoint-dev
go test ./...
```

MinIO console is at <http://localhost:9001> with the same credentials.

## Wire-thin discipline

Per Lazuli's founding principle, this adapter is **~180 LOC of import
+ call**. Each method maps to one AWS SDK call. Retries, circuit
breakers, content-type sniffing, and audit logging are runtime
concerns and stay out of the plugin.

## Error mapping

| Upstream | Plugin returns |
|---|---|
| `s3.NoSuchKey` / `NotFound` / `NoSuchBucket` | `storage.ErrFileNotFound` |
| `AccessDenied` / `InvalidAccessKeyId` / `SignatureDoesNotMatch` | `ErrUnauthorized` |
| Network / context | wrapped as-is |

## License

MIT — see [LICENSE](LICENSE).
