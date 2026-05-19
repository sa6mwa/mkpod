#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
WORKSPACE=${WORKSPACE:-$SCRIPT_DIR/workspace}
MKPOD_BIN=${MKPOD_BIN:-../bin/mkpod}
AWS_REGION=${AWS_REGION:-eu-west-1}
OUTPUT_BUCKET=${OUTPUT_BUCKET:-mkpod-integration-test}
INPUT_BUCKET=${INPUT_BUCKET:-mkpod-integration-test-assets}
TEST_SPEC=${TEST_SPEC:-$WORKSPACE/podspec-validation-test.yaml}
BASE_URL=${BASE_URL:-https://$OUTPUT_BUCKET.s3.$AWS_REGION.amazonaws.com}
RSS_CAPTURE=${RSS_CAPTURE:-$WORKSPACE/acceptance-validation.rss}

[ -x "$MKPOD_BIN" ] || { echo "error: mkpod binary not found at $MKPOD_BIN" >&2; exit 1; }
mkdir -p "$WORKSPACE"

assert_single_item_title() {
    python3 - "$1" "$2" "$3" <<'PY'
import sys
from pathlib import Path
import xml.etree.ElementTree as ET

rss_path, wanted_title, rejected_title = sys.argv[1:4]
content = Path(rss_path).read_text()
start = content.find('<rss')
if start < 0:
    raise SystemExit('FAIL: RSS payload missing from parse output')
root = ET.fromstring(content[start:])
channel = root.find('channel')
if channel is None:
    raise SystemExit('FAIL: RSS channel element missing')
items = channel.findall('item')
if len(items) != 1:
    raise SystemExit(f'FAIL: expected exactly 1 RSS item, found {len(items)}')
titles = [item.findtext('title', default='') for item in items]
if titles != [wanted_title]:
    raise SystemExit(f'FAIL: expected RSS item title {wanted_title!r}, got {titles!r}')
if rejected_title in titles:
    raise SystemExit(f'FAIL: unexpected invalid RSS item title {rejected_title!r} present')
PY
}

cat > "$TEST_SPEC" <<SPEC
config:
    baseURL: $BASE_URL
    image: $BASE_URL/artwork/podcast-cover.jpg
    defaultPodImage: artwork/podcast-cover.jpg
    aws:
        profile: default
        region: $AWS_REGION
        buckets:
            input: $INPUT_BUCKET
            output: $OUTPUT_BUCKET
    localStorageDir: $WORKSPACE
atom: podcast.rss
title: mkpod validation test
link: https://example.com/mkpod-validation
ttl: 60
language: en
copyright: Copyright mkpod validation test
webMaster: you@example.com
description: Validation test fixture.
subtitle: mkpod validation test
ownerName: Test Owner
ownerEmail: you@example.com
explicit: "no"
keywords: mkpod,validation,test
categories:
    - name: Technology
      subcategories: []
encoding:
    preferredFormat: m4a
    bitrate: 128
    lamepath: lame
    ffmpegpath: ffmpeg
    crf: 28
    abr: 128k
    coverfront: artwork/podcast-cover.jpg
    genre: Podcast
    language: eng
episodes:
    - uid: 1
      title: Valid Episode
      pubDate: Mon, 01 Sep 2025 21:21:04 +0000
      link: https://example.com/mkpod-validation/episodes/1
      duration: "00:02:00"
      author: "Episode Author"
      subtitle: This episode has all required fields
      description: A valid episode for validation testing
      type: "audio/mp4"
      length: 123456
      image: "artwork/podcast-cover.jpg"
      output: "episode-1.m4a"
    - uid: 2
      title: Missing Fields Episode
      pubDate: Tue, 02 Sep 2025 21:21:04 +0000
      link: https://example.com/mkpod-validation/episodes/2
      subtitle: Missing output, duration, image, type, and length
      description: This episode should be filtered out
SPEC

if "$MKPOD_BIN" parse --spec "$TEST_SPEC" --dry-run >/tmp/mkpod-validation-top-level.log 2>&1; then
    echo "FAIL: parse should have failed when top-level author was missing" >&2
    exit 1
fi

echo "PASS: top-level validation fails when required author is missing"

perl -0pi -e 's/ownerEmail: you\@example\.com\n/ownerEmail: you\@example.com\nauthor: Test Author\n/' "$TEST_SPEC"
output=$("$MKPOD_BIN" parse --spec "$TEST_SPEC" --dry-run 2>&1 || true)
printf '%s\n' "$output" > "$RSS_CAPTURE"
if [[ "$output" != *"Excluding episode from RSS due to missing required fields"* ]]; then
    echo "FAIL: expected episode filtering warning in parse output" >&2
    echo "$output" >&2
    exit 1
fi
assert_single_item_title "$RSS_CAPTURE" "Valid Episode" "Missing Fields Episode"

echo "PASS: parse excludes invalid episodes while still rendering valid ones"
rm -f "$TEST_SPEC" "$RSS_CAPTURE"
