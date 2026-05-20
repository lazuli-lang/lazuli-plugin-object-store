# `@plugin/object-store` vendor matrix

| Vendor | Auth | Region/endpoint config | Features supported | Limitations | Cost model |
|--------|------|------------------------|--------------------|-------------|------------|
| AWS S3 | IAM access key OR IRSA | `S3_REGION` | Put, Get, Delete, Sign, ListPrefix | None | per-GB stored plus per-1k requests |
| MinIO | access key | `S3_ENDPOINT` (self-hosted) | Put, Get, Delete, Sign, ListPrefix | Path-style URLs required | self-hosted infra cost |
| Cloudflare R2 | R2 access key | `S3_ENDPOINT = https://<account>.r2.cloudflarestorage.com` | Put, Get, Delete, Sign, ListPrefix | No multipart > 5 GiB on standard tier | zero egress fees |
| Tigris | access key | `S3_ENDPOINT = https://fly.storage.tigris.dev` | Put, Get, Delete, Sign, ListPrefix | Newer service; smaller global footprint | Fly.io-style metered |
| Backblaze B2 | application key | `S3_ENDPOINT = https://s3.<region>.backblazeb2.com` | Put, Get, Delete, Sign, ListPrefix | Some S3 features missing | low per-GB stored |

## Setup notes

- **AWS S3**: prefer IRSA over static keys when running on EKS.
- **MinIO**: set `S3_FORCE_PATH_STYLE=true` (path-style URLs required).
- **R2**: presigned URL TTL <= 7 days.
- **Tigris**: global replication on by default; costs scale with region count.
- **B2**: presigned URL signing is S3-compatible but not all metadata fields propagate.

## Switching vendors

The adapter is vendor-agnostic at the contract level. Switch vendor by:

1. Update `.env`: `S3_ENDPOINT` + `S3_ACCESS_KEY_ID` + `S3_SECRET_ACCESS_KEY` to the new vendor.
2. Optionally update `S3_REGION` (AWS) or `S3_FORCE_PATH_STYLE` (MinIO).
3. No code change required.
