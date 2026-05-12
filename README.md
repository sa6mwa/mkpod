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
- `mkpod preprocess` for optional raw audio cleanup before editing
- `mkpod new` to prepare a saved new episode plan
- `mkpod edit <plan.json>` or `mkpod new --edit <plan.json>` to revise a saved new episode plan
- `mkpod inspect <plan.json>` to inspect a saved plan
- `mkpod apply <plan.json>` to apply a saved plan locally
- `mkpod publish` to publish the generated feed and referenced assets

See [docs/architecture.md](docs/architecture.md) for the current simplified
package and boundary layout.

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

For the current podcast-focused defaults, AAC outputs use ffmpeg's `aac`
encoder at the configured `encoding.abr` target (default `128k`). `m4a`
and `m4b` remain simple container choices selected by episode `format` or
`encoding.preferredFormat`; there is no separate container-specific policy
layer yet.

S3 storage class handling is intentionally internal for now. mkpod keeps
the behavior in the S3 layer, but does not expose storage-class controls
as part of the main CLI workflow.

mkpod uses saved plan files for guided changes. `mkpod new` prepares a
`new.plan.json` artifact next to the selected spec file. `mkpod edit
<plan.json>` and `mkpod new --edit <plan.json>` reopen that saved new episode
plan and save the revised JSON back to the same path unless `--out` is set.
`mkpod inspect` shows what is in a saved plan, and `mkpod apply <plan.json>`
applies it. Applying a new episode plan updates `podspec.yaml`, encodes the
episode locally, and regenerates the local RSS file. Publishing remains an
explicit step with `mkpod publish`.

```console
$ mkpod new --non-interactive --title "Episode" --link https://example.com/episode --subtitle "Subtitle" --description "Description" --input masters/episode.flac
$ mkpod edit new.plan.json
$ mkpod inspect new.plan.json
$ mkpod apply new.plan.json
$ mkpod publish
```

## Quick Start

```console
$ mkpod init ./podcast
$ cd podcast
$ ls
artwork  masters  podspec.yaml

$ mkpod -h
Generate and encode podcasts and publish to a cloud object store

$ mkpod init --help
$ mkpod new --help
$ mkpod edit --help
$ mkpod inspect --help
$ mkpod apply --help

# Optional raw-track cleanup before editing
$ mkpod preprocess MIC1.WAV

# Prepare a new episode entry
$ mkpod new
$ mkpod edit new.plan.json
$ mkpod inspect new.plan.json
$ mkpod apply new.plan.json

# Encode a single episode selected by the uid field in podspec.yaml
$ mkpod encode 16

# Parse and upload podcast.rss
$ mkpod publish

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

If the target directory already exists and contains other files, use
`mkpod init --force <directory>` to regenerate `podspec.yaml` and create any
missing standard directories without deleting unrelated files.

`config.localStorageDir` is the workspace root by default. Encoded media files
and `podcast.rss` are written there unless you choose output paths that place
them in subdirectories.

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
