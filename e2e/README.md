# End-to-End Integration Tests

This directory contains end-to-end integration tests for mkpod that use real S3 buckets to test the complete workflow.

## S3 Buckets

- **mkpod-integration-test-assets** (input bucket): Contains artwork and master audio files
- **mkpod-integration-test** (output bucket): Contains encoded files and podcast.rss output

## Directory Structure

```
e2e/
├── Makefile          # Main test runner with complete workflow
├── README.md         # This file
├── pod/              # Test pod assets (artwork, audio files)
└── podspec.yaml      # Test podcast specification
```

## Setup

The setup is already complete with:
1. The `pod/` directory containing test assets
2. A `podspec.yaml` file configured for the S3 buckets

## Usage

### Run complete integration test workflow:
```bash
make test
```
This runs: setup → upload-pod → encode → upload-rss → verify-outputs

### Individual workflow steps:
```bash
make upload-pod       # Upload pod assets to input bucket
make encode           # Run mkpod encode (produces m4a files)
make generate-rss     # Generate podcast.rss from podspec.yaml
make upload-rss       # Upload podcast.rss to output bucket
make verify-outputs   # Check bucket contents and required files
```

### Bucket access control:
```bash
make make-public      # Make output bucket temporarily read-only public
make make-private     # Make output bucket private again
```

### Maintenance:
```bash
make clean           # Remove all test files from S3 buckets
make build-mkpod     # Build mkpod binary
make help            # Show all available targets
```

## Complete Test Flow

1. **Setup**: Builds mkpod and verifies S3 buckets exist
2. **Upload Pod Assets**: Syncs test assets (artwork, masters) to input bucket
3. **Encode**: Runs mkpod encode to produce m4a files, uploads to output bucket
4. **Generate RSS**: Creates podcast.rss from podspec.yaml with proper validation
5. **Upload RSS**: Uploads podcast.rss to output bucket
6. **Verification**: Checks that all required outputs exist in buckets

## Public Access Testing

After running the complete test, you can make the output bucket public to test RSS feed access:

```bash
make make-public
# Test your RSS feed at: https://mkpod-integration-test.s3.eu-west-1.amazonaws.com/podcast.rss
make make-private    # Make it private again when done
```

## Notes

- The tests use real S3 buckets and will incur AWS costs
- Always run `make clean` after testing to avoid unnecessary storage costs
- The `teardown` target will delete the S3 buckets entirely - use with caution
- Test files (.flac, .mp3, .m4a, .m4b, .wav, .jpeg) are ignored by git
- RSS validation uses `xmllint` to ensure proper format
