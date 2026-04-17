SHELL := /bin/bash

INPUT_BUCKET ?= mkpod-integration-test-assets
OUTPUT_BUCKET ?= mkpod-integration-test
AWS_PROFILE ?= default
AWS_REGION ?= eu-west-1
WORKSPACE ?= $(CURDIR)/workspace
MKPOD_BIN ?= ../bin/mkpod
BASE_URL ?= https://$(OUTPUT_BUCKET).s3.$(AWS_REGION).amazonaws.com
FEED_FILE := $(WORKSPACE)/podcast.rss
SPEC_FILE := $(WORKSPACE)/podspec.yaml
OUTPUT_BUCKET_JSON := $(WORKSPACE)/output-bucket.json

.PHONY: all help build-mkpod rebuild bootstrap-workspace verify-buckets setup \
	encode generate-rss upload-rss verify-outputs acceptance-aws \
	acceptance-remove-master acceptance-pubdate acceptance-validation \
	make-public make-private clean clean-local

all: help

build-mkpod:
	@if [ ! -x "$(MKPOD_BIN)" ]; then \
		echo "Building mkpod..."; \
		cd .. && make build; \
	else \
		echo "mkpod binary already exists at $(MKPOD_BIN)"; \
	fi

rebuild:
	@echo "Force rebuilding mkpod..."
	cd .. && make build

bootstrap-workspace: build-mkpod
	@INPUT_BUCKET="$(INPUT_BUCKET)" OUTPUT_BUCKET="$(OUTPUT_BUCKET)" AWS_PROFILE="$(AWS_PROFILE)" AWS_REGION="$(AWS_REGION)" BASE_URL="$(BASE_URL)" WORKSPACE="$(WORKSPACE)" MKPOD_BIN="$(MKPOD_BIN)" ./bootstrap-workspace.sh

verify-buckets:
	@aws s3 ls s3://$(INPUT_BUCKET) > /dev/null
	@aws s3 ls s3://$(OUTPUT_BUCKET) > /dev/null

setup: bootstrap-workspace verify-buckets
	@echo "AWS acceptance workspace ready at $(WORKSPACE)"

encode: setup
	$(MKPOD_BIN) encode --spec "$(SPEC_FILE)" --all --force

generate-rss: setup
	$(MKPOD_BIN) parse --spec "$(SPEC_FILE)" --force
	@test -f "$(FEED_FILE)"
	@if command -v xmllint >/dev/null 2>&1; then \
		xmllint --format "$(FEED_FILE)" > /dev/null; \
	fi

upload-rss: setup
	$(MKPOD_BIN) parse --spec "$(SPEC_FILE)" --upload --force
	@test -f "$(FEED_FILE)"

verify-outputs:
	@aws s3api list-objects-v2 --bucket $(OUTPUT_BUCKET) > $(OUTPUT_BUCKET_JSON)
	@python3 -c 'import json,sys; from pathlib import Path; payload=json.loads(Path(sys.argv[1]).read_text()); keys=[obj["Key"] for obj in payload.get("Contents", [])]; ("podcast.rss" in keys) or (_ for _ in ()).throw(SystemExit("FAIL: podcast.rss missing from output bucket")); any(key.endswith(".m4a") for key in keys) or (_ for _ in ()).throw(SystemExit("FAIL: no .m4a objects found in output bucket"))' "$(OUTPUT_BUCKET_JSON)"
	@rm -f $(OUTPUT_BUCKET_JSON)
	@echo "Verified podcast.rss and encoded audio in $(OUTPUT_BUCKET)"

acceptance-remove-master: setup
	@INPUT_BUCKET="$(INPUT_BUCKET)" OUTPUT_BUCKET="$(OUTPUT_BUCKET)" WORKSPACE="$(WORKSPACE)" SPEC_FILE="$(SPEC_FILE)" MKPOD_BIN="$(MKPOD_BIN)" ./acceptance-remove-master.sh

acceptance-pubdate: setup
	@cp "$(WORKSPACE)/original-podspec.yaml" "$(SPEC_FILE)"
	@perl -0pi -e 's/^pubDate:.*\n//m' "$(SPEC_FILE)"
	@$(MKPOD_BIN) parse --spec "$(SPEC_FILE)" --force
	@grep -q '^pubDate:' "$(SPEC_FILE)"
	@echo "Verified missing top-level pubDate is filled during parse"

acceptance-validation: setup
	@INPUT_BUCKET="$(INPUT_BUCKET)" OUTPUT_BUCKET="$(OUTPUT_BUCKET)" AWS_REGION="$(AWS_REGION)" WORKSPACE="$(WORKSPACE)" MKPOD_BIN="$(MKPOD_BIN)" ./acceptance-validation.sh

acceptance-aws: encode upload-rss verify-outputs acceptance-remove-master acceptance-pubdate acceptance-validation
	@echo "Completed AWS acceptance workflow"

make-public:
	aws s3api put-public-access-block --bucket $(OUTPUT_BUCKET) --public-access-block-configuration "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false"
	sed 's/BUCKET_NAME/$(OUTPUT_BUCKET)/g' bucket-policy.json > /tmp/bucket-policy-$(OUTPUT_BUCKET).json
	aws s3api put-bucket-policy --bucket $(OUTPUT_BUCKET) --policy file:///tmp/bucket-policy-$(OUTPUT_BUCKET).json
	rm -f /tmp/bucket-policy-$(OUTPUT_BUCKET).json

make-private:
	aws s3api delete-bucket-policy --bucket $(OUTPUT_BUCKET) || true
	aws s3api put-public-access-block --bucket $(OUTPUT_BUCKET) --public-access-block-configuration "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"

clean:
	aws s3 rm s3://$(OUTPUT_BUCKET) --recursive || true
	aws s3 rm s3://$(INPUT_BUCKET) --recursive || true
	@echo "Removed AWS acceptance objects"

clean-local:
	rm -rf "$(WORKSPACE)"

help:
	@echo "mkpod AWS acceptance targets (opt-in only)"
	@echo ""
	@echo "Variables:"
	@echo "  INPUT_BUCKET=$(INPUT_BUCKET)"
	@echo "  OUTPUT_BUCKET=$(OUTPUT_BUCKET)"
	@echo "  AWS_PROFILE=$(AWS_PROFILE)"
	@echo "  AWS_REGION=$(AWS_REGION)"
	@echo "  WORKSPACE=$(WORKSPACE)"
	@echo ""
	@echo "Targets:"
	@echo "  bootstrap-workspace      Run mkpod init and generate tiny local test fixtures"
	@echo "  setup                    Bootstrap workspace and verify S3 buckets"
	@echo "  encode                   Encode all episodes into the workspace and upload as needed"
	@echo "  generate-rss             Generate podcast.rss locally under the workspace"
	@echo "  upload-rss               Generate and upload podcast.rss"
	@echo "  verify-outputs           Assert that podcast.rss and encoded audio exist in S3"
	@echo "  acceptance-remove-master Verify remote master deletion safety checks"
	@echo "  acceptance-pubdate       Verify missing top-level pubDate gets filled"
	@echo "  acceptance-validation    Verify top-level validation and episode filtering"
	@echo "  acceptance-aws           Run the full opt-in AWS acceptance workflow"
	@echo "  clean                    Remove AWS objects from the acceptance buckets"
	@echo "  clean-local              Remove the generated local workspace"
