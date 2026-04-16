#!/bin/bash

# AWS acceptance test for mkpod validation behavior
# Tests both top-level field validation and episode filtering during RSS generation

set -e

MKPOD_BIN="../bin/mkpod"
ORIGINAL_PODSPEC="original-podspec.yaml"
TEST_PODSPEC="podspec-validation-test.yaml"

echo "=== Testing mkpod validation behavior ==="

# Create test podspec with missing required fields
echo "Creating test podspec with missing required top-level fields..."
cat > "$TEST_PODSPEC" << 'EOF'
config:
    baseURL: https://mkpod-integration-test.s3.eu-west-1.amazonaws.com
    image: https://mkpod-integration-test.s3.eu-west-1.amazonaws.com/artwork/qzj-1600x1600-english.jpeg
    defaultPodImage: artwork/qzj-3000x3000-english.jpeg
    aws:
        profile: default
        region: eu-west-1
        buckets:
            input: mkpod-integration-test-assets
            output: mkpod-integration-test
    localStorageDir: ./pod
atom: podcast.rss
title: mkpod validation test
link: https://qzj.se
ttl: 60
language: en
copyright: Copyright SA6MWA 2025 All Rights Reserved.
webMaster: sa6mwa@gmail.com
description: This is the mkpod validation test.
subtitle: mkpod validation test
ownerName: SA6MWA
ownerEmail: sa6mwa@gmail.com
# author: MISSING - should cause validation failure
explicit: "no"
keywords: mkpod,validation test,e2e test,test
categories:
    - name: Technology
      subcategories: []
encoding:
    preferredFormat: m4a
    bitrate: 128
    lamepath: /usr/bin/lame
    ffmpegpath: ~/bin/ffmpeg
    crf: 38
    abr: 128k
    coverfront: artwork/qzj-1000x1000-english.jpeg
    genre: Podcast
    language: end
episodes:
    - uid: 1
      title: Valid Episode
      pubDate: Mon, 01 Sep 2025 21:21:04 +0000
      link: https://qzj.se/audio/valid-episode
      duration: "00:10:30"
      author: "Episode Author"
      subtitle: This episode has all required fields
      description: A valid episode for testing
      type: "audio/mpeg"
      length: 1000000
      image: "episode1.jpg"
      input: masters/valid-episode.flac
      output: valid-episode.m4a
    - uid: 2
      title: Missing Fields Episode
      pubDate: Mon, 01 Sep 2025 21:21:04 +0000
      link: https://qzj.se/audio/missing-fields
      # duration: MISSING
      author: ""
      subtitle: This episode is missing duration, type, length, image fields
      description: An episode missing required fields
      # type: MISSING
      # length: MISSING
      # image: MISSING
      input: masters/missing-fields.flac
      # output: MISSING
    - uid: 3
      title: Episode Using Defaults
      pubDate: Mon, 01 Sep 2025 21:21:04 +0000
      link: https://qzj.se/audio/defaults-episode
      duration: "00:05:15"
      # author: empty, should use top-level default
      author: ""
      # explicit: empty, should use top-level default
      subtitle: This episode should use default author and explicit from top-level
      description: An episode that uses defaults
      type: "audio/mpeg"
      length: 500000
      image: "episode3.jpg"
      input: masters/defaults-episode.flac
      output: defaults-episode.m4a
EOF

echo "✓ Created test podspec with missing author field"

# Test 1: Top-level validation should fail
echo ""
echo "Test 1: Testing top-level field validation (should fail due to missing author)..."
if $MKPOD_BIN parse --spec "$TEST_PODSPEC" --dry-run 2>&1; then
    echo "✗ FAIL: Parse should have failed due to missing author field"
    exit 1
else
    echo "✓ PASS: Parse correctly failed due to missing required top-level fields"
fi

echo ""
echo "Adding missing author field to make top-level validation pass..."
sed -i 's/# author: MISSING - should cause validation failure/author: Test Author/' "$TEST_PODSPEC"
echo "✓ Added author field to test podspec"

# Test 2: Episode filtering during RSS generation
echo ""
echo "Test 2: Testing episode validation and filtering during RSS generation..."
echo "Running mkpod parse with --dry-run to see episode filtering..."
OUTPUT=$($MKPOD_BIN parse --spec "$TEST_PODSPEC" --dry-run 2>&1 || true)

if echo "$OUTPUT" | grep -q "Excluding episode from RSS due to missing required fields"; then
    echo "✓ PASS: Found expected warning about excluding invalid episodes"
else
    echo "✗ FAIL: Did not find expected episode filtering warnings"
    echo "Output was:"
    echo "$OUTPUT"
    exit 1
fi

if echo "$OUTPUT" | grep -q "uid=2.*Missing Fields Episode.*missingFields="; then
    echo "✓ PASS: Episode 2 (Missing Fields Episode) was correctly excluded with detailed field info"
else
    echo "✗ FAIL: Episode 2 filtering was not logged correctly"
    echo "Output was:"
    echo "$OUTPUT"
    exit 1
fi

if echo "$OUTPUT" | grep -q "Episode filtering complete.*excludedEpisodes=[1-9]"; then
    echo "✓ PASS: Found episode filtering summary"
else
    echo "✗ FAIL: Did not find episode filtering summary"
    echo "Output was:"
    echo "$OUTPUT"
    exit 1
fi

# Test 3: Verify that valid episodes appear in RSS output
echo ""
echo "Test 3: Testing that valid episodes appear in RSS output..."
RSS_OUTPUT=$($MKPOD_BIN parse --spec "$TEST_PODSPEC" --dry-run 2>/dev/null || true)

if echo "$RSS_OUTPUT" | grep -q "Valid Episode"; then
    echo "✓ PASS: Valid episode (UID 1) appears in RSS output"
else
    echo "✗ FAIL: Valid episode (UID 1) missing from RSS output"
    echo "RSS output snippet:"
    echo "$RSS_OUTPUT" | head -50
    exit 1
fi

if echo "$RSS_OUTPUT" | grep -q "Episode Using Defaults"; then
    echo "✓ PASS: Episode using defaults (UID 3) appears in RSS output"
else
    echo "✗ FAIL: Episode using defaults (UID 3) missing from RSS output"
    exit 1
fi

if echo "$RSS_OUTPUT" | grep -q "Missing Fields Episode"; then
    echo "✗ FAIL: Invalid episode (UID 2) should not appear in RSS output"
    exit 1
else
    echo "✓ PASS: Invalid episode (UID 2) correctly excluded from RSS output"
fi

# Test 4: Verify that default values are applied in RSS
echo ""
echo "Test 4: Testing that default values are correctly applied in RSS..."

if echo "$RSS_OUTPUT" | grep -q "<itunes:author>Test Author</itunes:author>"; then
    echo "✓ PASS: Default author is applied to episodes with empty author"
else
    echo "✗ FAIL: Default author not applied correctly"
    echo "Relevant RSS snippet:"
    echo "$RSS_OUTPUT" | grep -A5 -B5 "Episode Using Defaults" || echo "Episode not found"
    exit 1
fi

# Test 5: Verify episode count in RSS matches expected valid episodes
echo ""
echo "Test 5: Testing episode count in RSS matches expected valid episodes..."
EPISODE_COUNT=$(echo "$RSS_OUTPUT" | grep -c "<item>" || true)
if [ "$EPISODE_COUNT" = "2" ]; then
    echo "✓ PASS: RSS contains exactly 2 episodes (valid ones only)"
else
    echo "✗ FAIL: RSS contains $EPISODE_COUNT episodes, expected 2"
    echo "Episodes found in RSS:"
    echo "$RSS_OUTPUT" | grep -A2 "<item>" || echo "No items found"
    exit 1
fi

# Test 6: Test missing top-level field variations
echo ""
echo "Test 6: Testing various missing top-level fields..."

echo "Testing missing config.baseURL..."
sed -i 's/baseURL: https:/# baseURL: https:/' "$TEST_PODSPEC"
if $MKPOD_BIN parse --spec "$TEST_PODSPEC" --dry-run >/dev/null 2>&1; then
    echo "✗ FAIL: Should have failed due to missing baseURL"
    exit 1
else
    echo "✓ PASS: Correctly failed due to missing baseURL"
fi

sed -i 's/# baseURL: https:/baseURL: https:/' "$TEST_PODSPEC"

echo "Testing missing title..."
sed -i 's/title: mkpod validation test/# title: mkpod validation test/' "$TEST_PODSPEC"
if $MKPOD_BIN parse --spec "$TEST_PODSPEC" --dry-run >/dev/null 2>&1; then
    echo "✗ FAIL: Should have failed due to missing title"
    exit 1
else
    echo "✓ PASS: Correctly failed due to missing title"
fi

sed -i 's/# title: mkpod validation test/title: mkpod validation test/' "$TEST_PODSPEC"

echo ""
echo "Cleaning up test files..."
rm -f "$TEST_PODSPEC"
rm -f podcast.rss

echo ""
echo "=== All validation behavior tests passed! ==="
