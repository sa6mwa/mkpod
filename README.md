# mkpod

`mkpod` is a CLI to help automate publishing an audio and/or video podcast to
an Amazon S3 bucket. It uses the AWS Go SDK and depends on host-provided
`ffmpeg`, `ffprobe`, and `lame` for media inspection and encoding.

The CLI uses the input from a YAML configuration file called `podspec.yaml` (an
example is provided). AWS configuration, output directories, metadata and
information about the episodes are all entered into `podspec.yaml`. The
intention is for you to store this file in a private VCS (could also be public,
does not contain credentials).

The main commands are:

- `mkpod init <directory>` to create a starter workspace and `podspec.yaml`
- `mkpod preprocess` to run microphone/raw audio through ffmpeg filters
- `mkpod encode` to encode and upload episode media
- `mkpod parse` to generate `podcast.rss` and optionally upload it

`mkpod` encodes audio or video *masters* into `mp4` or `mp3`. If the input and
output is `audio`, `lame` will be used to create an `mp3`. If the input
content-type starts with `video/` and the `format` field for the episode is not
set to `audio`, the output will be `mp4` encoded with `ffmpeg`. If the input
content-type starts with `video/` and the `format` field for the episode is set
to `audio`, `ffmpeg` will be used to extract the audio as `pcm_s16le` (`wav`)
piped into `lame` stored as an `mp3` (without the video stream, the episode
will be an audio-only episode).

For AAC-based outputs (`m4a`, `m4b`, and MP4 audio tracks), mkpod now uses
ffmpeg's built-in `aac` encoder rather than `libfdk_aac`.

## Quick Start

```console
$ mkpod init ./podcast
$ cd podcast
$ ls
artwork  masters  podspec.yaml

$ mkpod -h
Generate and encode podcasts and publish to a cloud object store

$ mkpod init --help
$ mkpod parse --help
$ mkpod encode --help

# Pre-process a raw microphone track
$ mkpod pre MIC1.WAV

# Encode all episodes in podspec.yaml
$ mkpod e -a

# Encode a single episode selected by the uid field in podspec.yaml
$ mkpod e 16

# Parse and upload podcast.rss
$ mkpod p -u

# Commit changes to podspec.yaml
$ git add podspec.yaml ; git commit -m 'Update pod' ; git push
```

## Host Dependencies

Install these tools from your OS or distribution packages:

- `ffmpeg`
- `ffprobe`
- `lame`

By default mkpod looks them up on `PATH`. You can still override the binary
paths in `podspec.yaml` via `encoding.ffmpegpath` and `encoding.lamepath` if
needed.

## Minimal `podspec.yaml`

```yaml
config:
  baseURL: https://example-podcast-bucket.s3.us-east-1.amazonaws.com
  image: https://example-podcast-bucket.s3.us-east-1.amazonaws.com/artwork/podcast-cover.jpg
  defaultPodImage: artwork/podcast-cover.jpg
  aws:
    profile: default
    region: us-east-1
    buckets:
      input: example-podcast-assets
      output: example-podcast-bucket
  localStorageDir: /absolute/path/to/your/podcast
atom: podcast.rss
title: my-podcast
link: https://example.com/my-podcast
ttl: 60
language: en
copyright: Copyright Example
webMaster: you@example.com
description: Replace this description with your podcast summary.
subtitle: Replace this subtitle.
ownerName: Your Name
ownerEmail: you@example.com
author: Your Name
explicit: "no"
keywords: podcast
categories:
  - name: Technology
    subcategories: []
encoding:
  preferredFormat: mp3
  bitrate: 128
  lamepath: lame
  ffmpegpath: ffmpeg
  crf: 28
  abr: 128k
  coverfront: artwork/podcast-cover.jpg
  genre: Podcast
  language: eng
episodes: []
```

Use `mkpod init <directory>` to generate this starter layout automatically and
then edit the values for your real podcast and S3 buckets.

## AWS access policy

For the public podcast bucket, you are going to have to disable `Block
all public access` (set it to `Off`) under `Permissions` in the
console or via...

```console
aws s3api put-public-access-block \
	--bucket YOUR_BUCKET_NAME \
	--public-access-block-configuration \
	BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false
```

Apply something like the following policy to allow everyone to
download the rss, audio, artwork, etc. List-access is not required.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PublicRead",
            "Effect": "Allow",
            "Principal": {
                "AWS": "*"
            },
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::mypodbucket/*"
        }
    ]
}
```

Apply the policy in the UI or via something like the following
command...

```console
aws s3api put-bucket-policy --bucket YOUR_BUCKET_NAME --policy file://YOUR_POLICY_FILE.json
```
