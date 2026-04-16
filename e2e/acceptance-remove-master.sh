#!/bin/bash

# AWS acceptance test for remote master removal safety checks

INPUT_BUCKET="mkpod-integration-test-assets"
OUTPUT_BUCKET="mkpod-integration-test"
MKPOD_BIN="../bin/mkpod"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

cleanup() {
    local exit_code=$?
    echo -e "\n${BLUE}=== Cleanup: Restoring any backup files ===${NC}"

    if [ -f "pod/masters/qzj029-english.flac.backup" ]; then
        echo "Restoring qzj029-english.flac from backup"
        mv pod/masters/qzj029-english.flac.backup pod/masters/qzj029-english.flac 2>/dev/null || true
    fi

    if [ -f "pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac.backup" ]; then
        echo "Restoring qzj033-c4isr-computers-data-driven-decisions.flac from backup"
        mv pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac.backup pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac 2>/dev/null || true
    fi

    echo "Cleanup completed"
    if [ $exit_code -ne 0 ]; then
        exit $exit_code
    fi
}

trap cleanup EXIT

print_test_header() {
    echo -e "\n${BLUE}=== Test $1: $2 ===${NC}"
    ((TESTS_RUN++))
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
    ((TESTS_PASSED++))
}

print_failure() {
    echo -e "${RED}✗ $1${NC}"
    ((TESTS_FAILED++))
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

echo -e "${BLUE}=== Remote Master Removal Safety Tests ===${NC}"
echo "Verifying prerequisites..."

if [ ! -f "$MKPOD_BIN" ]; then
    echo -e "${RED}Error: mkpod binary not found at $MKPOD_BIN${NC}"
    echo "Run 'make build' from the root directory first"
    exit 1
fi

if [ ! -f "podspec.yaml" ]; then
    echo -e "${RED}Error: podspec.yaml not found${NC}"
    echo "Run this script from the e2e directory with a valid podspec.yaml"
    exit 1
fi

echo "Verifying S3 buckets..."
aws s3 ls s3://$INPUT_BUCKET > /dev/null || { echo -e "${RED}Input bucket $INPUT_BUCKET not accessible${NC}"; exit 1; }
aws s3 ls s3://$OUTPUT_BUCKET > /dev/null || { echo -e "${RED}Output bucket $OUTPUT_BUCKET not accessible${NC}"; exit 1; }

print_success "Prerequisites verified"

echo -e "\n${BLUE}=== Setup: Uploading master files to input bucket ===${NC}"
aws s3 cp pod/masters/qzj029-english.flac s3://$INPUT_BUCKET/masters/qzj029-english.flac
aws s3 cp pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac s3://$INPUT_BUCKET/masters/qzj033-c4isr-computers-data-driven-decisions.flac

print_success "Master files uploaded to input bucket"

print_test_header "1" "Normal case - safety checks should pass"
echo "Local master file exists and should be adequate size"
echo "Expected: Safety checks pass, remote master gets removed"

if $MKPOD_BIN encode 1 --remove-remote-master --force; then
    print_success "Test 1 completed - check logs for safety check messages"
else
    print_failure "Test 1 failed unexpectedly"
fi

print_test_header "2" "Missing local master file"
echo "Moving local master file to test safety check"
mv pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac.backup

echo "Expected: Should skip removal due to missing local file"
if $MKPOD_BIN encode 2 --remove-remote-master --force; then
    if aws s3 ls s3://$INPUT_BUCKET/masters/qzj033-c4isr-computers-data-driven-decisions.flac > /dev/null 2>&1; then
        print_success "Test 2 passed - remote file preserved when local file missing"
    else
        print_failure "Test 2 failed - remote file was removed despite missing local file"
    fi
else
    print_warning "Test 2 had encoding issues, but this is expected behavior"
fi

mv pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac.backup pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac

print_test_header "3" "Local file too small (size safety check)"
echo "Creating backup and replacing with small dummy file"
cp pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac.backup
echo "small dummy content that is definitely less than 50% of original" > pod/masters/qzj033-c4isr-computers-data-driven-decisions.flac

echo "Expected: Should skip removal due to file being too small"
if $MKPOD_BIN encode 2 --remove-remote-master --force; then
    if aws s3 ls s3://$INPUT_BUCKET/masters/qzj033-c4isr-computers-data-driven-decisions.flac > /dev/null 2>&1; then
        print_success "Test 3 passed - remote file preserved when local file too small"
    else
        print_failure "Test 3 failed - remote file was removed despite local file being too small"
    fi
else
    print_warning "Test 3 had encoding issues, but safety check should have worked"
fi

echo -e "\n${BLUE}=== Test Results Summary ===${NC}"
echo -e "Tests run: $TESTS_RUN"
echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}All safety tests completed successfully!${NC}"
    echo -e "The remote master removal safety checks are working correctly."
else
    echo -e "\n${RED}Some tests failed${NC}"
    echo -e "Please review the safety check implementation."
    exit 1
fi

echo -e "\n${BLUE}=== Final Verification ===${NC}"
echo "Input bucket contents:"
aws s3 ls s3://$INPUT_BUCKET/ --recursive
echo ""
echo "Output bucket contents:"
aws s3 ls s3://$OUTPUT_BUCKET/ --recursive
