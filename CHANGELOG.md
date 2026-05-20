# Changelog

All notable changes to `@plugin/object-store` will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-05-20

### Added
- Initial release: S3-compatible object-store adapter for AWS S3, MinIO, Cloudflare R2, Tigris, and Backblaze B2.
- Go server adapter implementing `storage.ObjectStore` (see `manifest.toml`).
- Auto-registers via `init()` against `@plugin/object-store`.

### Vendor SDK pin
- `github.com/aws/aws-sdk-go-v2/service/s3` v1.101.0
