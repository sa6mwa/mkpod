#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
WORKSPACE=${WORKSPACE:-$SCRIPT_DIR/workspace}
INPUT_BUCKET=${INPUT_BUCKET:-mkpod-integration-test-assets}
OUTPUT_BUCKET=${OUTPUT_BUCKET:-mkpod-integration-test}
MKPOD_BIN=${MKPOD_BIN:-../bin/mkpod}
SPEC_FILE=${SPEC_FILE:-$WORKSPACE/podspec.yaml}
MASTER_ONE=masters/episode-1.wav
MASTER_TWO=masters/episode-2.wav
LOCAL_MASTER_TWO=$WORKSPACE/$MASTER_TWO
BACKUP_MASTER_TWO=$LOCAL_MASTER_TWO.backup

cleanup() {
    if [ -f "$BACKUP_MASTER_TWO" ]; then
        mv "$BACKUP_MASTER_TWO" "$LOCAL_MASTER_TWO"
    fi
}
trap cleanup EXIT

[ -x "$MKPOD_BIN" ] || { echo "error: mkpod binary not found at $MKPOD_BIN" >&2; exit 1; }
[ -f "$SPEC_FILE" ] || { echo "error: spec file not found at $SPEC_FILE" >&2; exit 1; }

aws s3 cp "$WORKSPACE/$MASTER_ONE" "s3://$INPUT_BUCKET/$MASTER_ONE" >/dev/null
aws s3 cp "$WORKSPACE/$MASTER_TWO" "s3://$INPUT_BUCKET/$MASTER_TWO" >/dev/null

"$MKPOD_BIN" encode --spec "$SPEC_FILE" 1 --remove-remote-master --force >/dev/null
if aws s3 ls "s3://$INPUT_BUCKET/$MASTER_ONE" >/dev/null 2>&1; then
    echo "FAIL: remote master for episode 1 should have been removed" >&2
    exit 1
fi

echo "PASS: remote master was removed when local master existed and size checks passed"

aws s3 cp "$WORKSPACE/$MASTER_TWO" "s3://$INPUT_BUCKET/$MASTER_TWO" >/dev/null
mv "$LOCAL_MASTER_TWO" "$BACKUP_MASTER_TWO"
"$MKPOD_BIN" encode --spec "$SPEC_FILE" 2 --remove-remote-master --force >/dev/null || true
if ! aws s3 ls "s3://$INPUT_BUCKET/$MASTER_TWO" >/dev/null 2>&1; then
    echo "FAIL: remote master for episode 2 was removed even though the local master was missing" >&2
    exit 1
fi

echo "PASS: remote master was preserved when the local master was missing"

mv "$BACKUP_MASTER_TWO" "$LOCAL_MASTER_TWO"
cp "$LOCAL_MASTER_TWO" "$BACKUP_MASTER_TWO"
printf 'tiny\n' > "$LOCAL_MASTER_TWO"
aws s3 cp "$BACKUP_MASTER_TWO" "s3://$INPUT_BUCKET/$MASTER_TWO" >/dev/null
"$MKPOD_BIN" encode --spec "$SPEC_FILE" 2 --remove-remote-master --force >/dev/null || true
if ! aws s3 ls "s3://$INPUT_BUCKET/$MASTER_TWO" >/dev/null 2>&1; then
    echo "FAIL: remote master for episode 2 was removed even though the local file was too small" >&2
    exit 1
fi

echo "PASS: remote master was preserved when the local master failed the size check"
