# End-to-end integration tests for mkpod
# Uses real S3 buckets for testing complete workflows

# S3 bucket configuration (matching podspec.yaml terminology)
INPUT_BUCKET := mkpod-integration-test-assets
OUTPUT_BUCKET := mkpod-integration-test

# Build mkpod if it doesn't exist
MKPOD_BIN := ../bin/mkpod

.PHONY: all clean test setup teardown build-mkpod rebuild encode generate-rss upload-rss make-public make-private verify-outputs test-remove-master-safety test-pubdate-behavior

all: test

# Build mkpod binary only if it doesn't exist
build-mkpod:
	@if [ ! -f "$(MKPOD_BIN)" ]; then \
		echo "Building mkpod..."; \
		cd .. && make build; \
	else \
		echo "mkpod binary already exists at $(MKPOD_BIN)"; \
	fi

# Force rebuild mkpod binary
rebuild:
	@echo "Force rebuilding mkpod..."
	cd .. && make build

# Setup test environment
setup: build-mkpod
	@echo "Setting up e2e test environment..."
	@echo "Verifying S3 buckets exist..."
	aws s3 ls s3://$(INPUT_BUCKET) > /dev/null
	aws s3 ls s3://$(OUTPUT_BUCKET) > /dev/null
	@echo "S3 buckets verified"

# Run mkpod encode to produce m4a files and upload to output bucket
# mkpod will handle uploading masters and artwork to input bucket as needed
encode: setup
	@echo "Running mkpod encode..."
	@if [ ! -f "podspec.yaml" ]; then \
		echo "No podspec.yaml found. Please create one with your test configuration."; \
		exit 1; \
	fi
	$(MKPOD_BIN) encode --spec podspec.yaml --all
	@echo "Encoding completed"

# Generate podcast.rss from podspec.yaml
generate-rss: setup
	@echo "Generating podcast.rss from podspec.yaml..."
	@if [ ! -f "podspec.yaml" ]; then \
		echo "No podspec.yaml found. Please create one with your test configuration."; \
		exit 1; \
	fi
	$(MKPOD_BIN) parse --spec podspec.yaml
	@echo "RSS generation completed"
	@if [ -f "podcast.rss" ]; then \
		echo "Validating RSS format..."; \
		xmllint --format podcast.rss > /dev/null && echo "RSS format is valid" || echo "WARNING: RSS format may be invalid"; \
	else \
		echo "ERROR: podcast.rss was not generated"; \
		exit 1; \
	fi

# Generate and upload podcast.rss to output bucket in one step
upload-rss: setup
	@echo "Generating and uploading podcast.rss to output bucket..."
	@if [ ! -f "podspec.yaml" ]; then \
		echo "No podspec.yaml found. Please create one with your test configuration."; \
		exit 1; \
	fi
	$(MKPOD_BIN) parse --spec podspec.yaml --upload --force
	@echo "RSS generation and upload completed"
	@if [ -f "podcast.rss" ]; then \
		echo "Validating RSS format..."; \
		xmllint --format podcast.rss > /dev/null && echo "RSS format is valid" || echo "WARNING: RSS format may be invalid"; \
	else \
		echo "ERROR: podcast.rss was not generated locally"; \
		exit 1; \
	fi

# Make output bucket temporarily read-only public
make-public:
	@echo "Making output bucket temporarily read-only public..."
	@echo "Disabling Block Public Access settings..."
	aws s3api put-public-access-block --bucket $(OUTPUT_BUCKET) --public-access-block-configuration "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false"
	@echo "Applying public read policy..."
	sed 's/BUCKET_NAME/$(OUTPUT_BUCKET)/g' bucket-policy.json > /tmp/bucket-policy-$(OUTPUT_BUCKET).json
	aws s3api put-bucket-policy --bucket $(OUTPUT_BUCKET) --policy file:///tmp/bucket-policy-$(OUTPUT_BUCKET).json
	rm -f /tmp/bucket-policy-$(OUTPUT_BUCKET).json
	@echo "Bucket $(OUTPUT_BUCKET) is now read-only public"
	@echo "Access via: https://$(OUTPUT_BUCKET).s3.eu-west-1.amazonaws.com/podcast.rss"

# Make output bucket private again
make-private:
	@echo "Making output bucket private again..."
	aws s3api delete-bucket-policy --bucket $(OUTPUT_BUCKET) || echo "No bucket policy to delete"
	@echo "Re-enabling Block Public Access settings for security..."
	aws s3api put-public-access-block --bucket $(OUTPUT_BUCKET) --public-access-block-configuration "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
	@echo "Bucket $(OUTPUT_BUCKET) is now private and secure"

# Verify outputs in S3 buckets
verify-outputs:
	@echo "Verifying outputs in S3 buckets..."
	@echo "Input bucket contents:"
	aws s3 ls s3://$(INPUT_BUCKET)/ --recursive
	@echo ""
	@echo "Output bucket contents:"
	aws s3 ls s3://$(OUTPUT_BUCKET)/ --recursive
	@echo ""
	@echo "Checking for required files..."
	@aws s3 ls s3://$(OUTPUT_BUCKET)/podcast.rss > /dev/null && echo "✓ podcast.rss found" || echo "✗ podcast.rss missing"
	@aws s3 ls s3://$(OUTPUT_BUCKET)/ | grep -q "\.m4a" && echo "✓ m4a files found" || echo "✗ m4a files missing"

# Test remote master removal safety checks
test-remove-master-safety: setup
	@echo "Running remote master removal safety tests..."
	./test-remove-master-safety.sh

# Test pubDate field filling behavior
test-pubdate-behavior: setup
	@echo "Testing pubDate field filling behavior..."
	@echo "Starting with original podspec (no pubDate)..."
	if [ -f "original-podspec.yaml" ]; then \
		cp original-podspec.yaml podspec.yaml; \
		echo "Reset podspec.yaml from original-podspec.yaml"; \
	else \
		echo "ERROR: original-podspec.yaml not found"; \
		exit 1; \
	fi
	@echo "Checking that original podspec has no pubDate or has zero date..."
	if grep -q "pubDate:" podspec.yaml; then \
		echo "Original podspec contains pubDate field - checking if it's empty or zero"; \
	else \
		echo "Original podspec has no pubDate field - good for testing"; \
	fi
	@echo "Running mkpod parse to fill in missing pubDate..."
	$(MKPOD_BIN) parse --spec podspec.yaml
	@echo "Verifying that pubDate was filled with current time..."
	if grep -q "pubDate: " podspec.yaml && ! grep -q "Mon, 01 Jan 0001 00:00:00 +0000" podspec.yaml; then \
		echo "✓ pubDate successfully filled with current time"; \
	else \
		echo "✗ pubDate was not properly filled"; \
		exit 1; \
	fi
	@echo "pubDate behavior test completed successfully"

# Run complete integration test workflow
test: setup encode upload-rss verify-outputs test-remove-master-safety test-pubdate-behaviour
	@echo "Running complete e2e integration test workflow..."
	@echo "Complete mkpod workflow test completed successfully!"
	@echo ""
	@echo "To make the bucket public for testing, run: make make-public"
	@echo "To make it private again, run: make make-private"

# Clean up test outputs from S3 buckets
clean:
	@echo "Cleaning up test outputs..."
	@echo "Removing all objects from output bucket..."
	aws s3 rm s3://$(OUTPUT_BUCKET) --recursive || true
	@echo "Removing all objects from input bucket..."
	aws s3 rm s3://$(INPUT_BUCKET) --recursive || true
	@echo "Resetting podspec.yaml to original state for field filling tests..."
	if [ -f "original-podspec.yaml" ]; then \
		cp original-podspec.yaml podspec.yaml; \
		echo "podspec.yaml reset from original-podspec.yaml"; \
	else \
		echo "WARNING: original-podspec.yaml not found, podspec.yaml not reset"; \
	fi
	@echo "Cleanup completed"

# Completely remove the S3 buckets (use with caution)
teardown: clean
	@echo "WARNING: This will delete the S3 buckets entirely!"
	@read -p "Are you sure you want to delete the buckets? (y/N): " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		aws s3 rb s3://$(INPUT_BUCKET) --force; \
		aws s3 rb s3://$(OUTPUT_BUCKET) --force; \
		echo "S3 buckets deleted"; \
	else \
		echo "Teardown cancelled"; \
	fi

help:
	@echo "E2E Integration Test Makefile for mkpod"
	@echo ""
	@echo "Available targets:"
	@echo "  setup         - Build mkpod and verify S3 buckets exist"
	@echo "  encode        - Run mkpod encode (handles all uploads)"
	@echo "  generate-rss  - Generate podcast.rss from podspec.yaml"
	@echo "  upload-rss    - Upload podcast.rss to output bucket"
	@echo "  make-public   - Make output bucket temporarily read-only public"
	@echo "  make-private  - Make output bucket private again"
	@echo "  verify-outputs- Verify outputs in S3 buckets"
	@echo "  test          - Run complete e2e integration workflow"
	@echo "  test-remove-master-safety - Test --remove-remote-master safety checks"
	@echo "  test-pubdate-behavior - Test pubDate field filling behavior"
	@echo "  clean         - Remove all test files from S3 buckets"
	@echo "  teardown      - Delete S3 buckets entirely (use with caution)"
	@echo "  build-mkpod   - Build the mkpod binary (only if missing)"
	@echo "  rebuild       - Force rebuild the mkpod binary"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "S3 Buckets:"
	@echo "  Input:   $(INPUT_BUCKET)"
	@echo "  Output:  $(OUTPUT_BUCKET)"
