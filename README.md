# mkpod

`mkpod` is a CLI to help automate publishing an audio and/or video podcast to
an Amazon S3 bucket. The CLI comes with the Amazon Go SDK, but depends on the
external tools `ffmpeg` and `lame` to encode masters to pod content.

The CLI uses the input from a YAML configuration file called `podspec.yaml` (an
example is provided). AWS configuration, output directories, meta data and
information about the episodes are all entered into `podcast.yaml`. The
intention is for you to store this file in a private VCS (could also be public,
does not contain credentials).

Each process has it's own sub-command, there are currently two sub-commands:
`encode` and `parse`. Encoding produces output files (`mp3` or `mp4`) and
uploads them to the `output` AWS S3 bucket specified in `podspec.yaml` while
the `parse` sub-command produces a `podcast.rss` XML file compatible with the
Apple Podcast XML format. Running `mkpod p -u` will both parse and allow you to
upload the `podcast.rss` file to the `output` AWS S3 bucket effectively
updating the podcast feed.

`mkpod` encodes audio or video *masters* into `mp4` or `mp3`. If the input and
output is `audio`, `lame` will be used to create an `mp3`. If the input
content-type starts with `video/` and the `format` field for the episode is not
set to `audio`, the output will be `mp4` encoded with `ffmpeg`. If the input
content-type starts with `video/` and the `format` field for the episode is set
to `audio`, `ffmpeg` will be used to extract the audio as `pcm_s16le` (`wav`)
piped into `lame` stored as an `mp3` (without the video stream, the episode
will be an audio-only episode).

## Example

```console
$ ls
podspec.yaml

$ mkpod -h
NAME:
   mkpod - Tool to render a podcast rss feed from spec, automate mp3/mp4 encoding and publish to Amazon S3.

USAGE:
   mkpod [global options] command [command options]

COMMANDS:
   preprocess, pre  Run an audiofile (e.g a raw microphone track) through pre-processing
   parse, p         Parse Go template using specification yaml
   encode, e        Encode and upload single or all output files in podspec.yaml
   help, h          Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help

COPYRIGHT:
   Copyright SA6MWA 2022-2023 sa6mwa@gmail.com, https://github.com/sa6mwa/mkpod

# Pre-process raw microphone track
$ mkpod pre --profile qzj MIC1.WAV

# Encode all episodes in podspec.yaml
$ mkpod e -a

# Encode a single episode selected by the uid field in podspec.yaml
$ mkpod e 16

# Parse and upload podcast.rss
$ mkpod p -u

# Commit changes to podspec.yaml
$ git add podspec.yaml ; git commit -m 'Update pod' ; git push
```

## Build ffmpeg with libfdk_aac

`mkpod` uses `libfdk_aac` to encode `mp4`. In the [scripts/](scripts) directory
you will find a build script that should work for various Ubuntu/Debian Linux
distributions.

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
