#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
WORKSPACE=${WORKSPACE:-$SCRIPT_DIR/workspace}
INPUT_BUCKET=${INPUT_BUCKET:-mkpod-integration-test-assets}
OUTPUT_BUCKET=${OUTPUT_BUCKET:-mkpod-integration-test}
MKPOD_BIN=${MKPOD_BIN:-../bin/mkpod}
SPEC_FILE=${SPEC_FILE:-$WORKSPACE/podspec.yaml}
MASTER_ONE=masters/episode-1.wav
IMAGE_ONE=artwork/episode-1.jpg
LOCAL_MASTER_ONE=$WORKSPACE/$MASTER_ONE
LOCAL_IMAGE_ONE=$WORKSPACE/$IMAGE_ONE
BACKUP_MASTER_ONE=$LOCAL_MASTER_ONE.backup
BACKUP_IMAGE_ONE=$LOCAL_IMAGE_ONE.backup

cleanup() {
    if [ -f "$BACKUP_MASTER_ONE" ]; then
        mv "$BACKUP_MASTER_ONE" "$LOCAL_MASTER_ONE"
    fi
    if [ -f "$BACKUP_IMAGE_ONE" ]; then
        mv "$BACKUP_IMAGE_ONE" "$LOCAL_IMAGE_ONE"
    fi
}
trap cleanup EXIT

[ -x "$MKPOD_BIN" ] || { echo "error: mkpod binary not found at $MKPOD_BIN" >&2; exit 1; }
[ -f "$SPEC_FILE" ] || { echo "error: spec file not found at $SPEC_FILE" >&2; exit 1; }

aws s3 cp "$WORKSPACE/$MASTER_ONE" "s3://$INPUT_BUCKET/$MASTER_ONE" >/dev/null
mv "$LOCAL_MASTER_ONE" "$BACKUP_MASTER_ONE"
"$MKPOD_BIN" encode --spec "$SPEC_FILE" 1 >/dev/null
[ -f "$LOCAL_MASTER_ONE" ] || { echo "FAIL: encode did not download missing remote master" >&2; exit 1; }
[ -f "$WORKSPACE/episode-1.m4a" ] || { echo "FAIL: encode did not produce output after downloading remote master" >&2; exit 1; }
echo "PASS: encode downloads remote master when local master is missing"

aws s3 cp "$WORKSPACE/$IMAGE_ONE" "s3://$OUTPUT_BUCKET/$IMAGE_ONE" >/dev/null
mv "$LOCAL_IMAGE_ONE" "$BACKUP_IMAGE_ONE"
"$MKPOD_BIN" parse --spec "$SPEC_FILE" --upload --force >/dev/null
[ -f "$LOCAL_IMAGE_ONE" ] || { echo "FAIL: parse did not download missing remote episode image" >&2; exit 1; }
echo "PASS: parse downloads remote episode image when local image is missing"

mv "$LOCAL_IMAGE_ONE" "$BACKUP_IMAGE_ONE"
aws s3 rm "s3://$OUTPUT_BUCKET/$IMAGE_ONE" >/dev/null
if "$MKPOD_BIN" parse --spec "$SPEC_FILE" --upload --force >/tmp/mkpod-fallbacks.log 2>&1; then
    echo "FAIL: parse unexpectedly succeeded when episode image was missing locally and remotely" >&2
    exit 1
fi
if ! grep -q "missing episode 1 image" /tmp/mkpod-fallbacks.log; then
    echo "FAIL: parse failure did not mention missing episode image" >&2
    cat /tmp/mkpod-fallbacks.log >&2
    exit 1
fi
echo "PASS: parse fails when an episode image is missing locally and remotely"
rm -f /tmp/mkpod-fallbacks.log
