# AWS Acceptance Tests

This directory contains opt-in AWS acceptance tests for `mkpod`. They are not part of `go test ./...` and they assume you have real S3 buckets you are willing to write to.

## What changed

The old `e2e/` setup assumed a checked-in `pod/` fixture tree that no longer exists. The acceptance flow now bootstraps a fresh workspace by running `mkpod init`, then generates tiny local artwork and WAV masters with host `ffmpeg`.

That gives the AWS tests a reproducible starting point without requiring large checked-in media fixtures.

## Requirements

- `ffmpeg`
- AWS CLI configured for the target profile
- writable S3 buckets for `INPUT_BUCKET` and `OUTPUT_BUCKET`
- a built `mkpod` binary, or let `make` build `../bin/mkpod`

## Common commands

```bash
cd e2e
make help
make bootstrap-workspace
make acceptance-validation
make acceptance-aws INPUT_BUCKET=my-input OUTPUT_BUCKET=my-output AWS_REGION=us-east-1
```

## Workspace layout

By default the workspace is generated under `e2e/workspace`.

`make bootstrap-workspace` will:
- run `mkpod init`
- generate `artwork/podcast-cover.jpg`
- generate `masters/episode-1.wav` and `masters/episode-2.wav`
- write a bucket-specific `podspec.yaml`
- save a copy as `original-podspec.yaml` for the pubDate acceptance check

## Acceptance scope

- `acceptance-validation`: top-level validation plus episode filtering during RSS rendering
- `acceptance-pubdate`: filling a missing top-level `pubDate`
- `acceptance-remove-master`: safety checks around `--remove-remote-master`
- `acceptance-aws`: end-to-end encode, RSS upload, and acceptance checks
- `acceptance-fallbacks`: local-first fallback behavior for remote masters and images

## Notes

- These tests intentionally stay outside the default local test loop.
- `make clean` only removes remote AWS objects.
- `make clean-local` removes the generated local workspace.
