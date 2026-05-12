# mkpod Workflow Map

This document maps the pre-`mkpod new` / `mkpod apply` workflow as implemented
by the legacy commands and the supporting storage/encoding code. It is a factual
map, not a redesign proposal.

Scope:

- "Original workflow" means the manual episode workflow centered on editing
  `podspec.yaml`, then running `mkpod encode`, then running `mkpod parse
  --upload` when ready to publish.
- The original public commands are now hidden in help, but their code paths
  still exist as `encode` and `parse`.
- This map also notes where later refactors changed or obscured original
  behavior, because those differences matter for rebuilding `new` / `apply`
  correctly.

## Idempotency Caveat

All workflow commands should be idempotent.

The original flow was nearly idempotent because most side effects were guarded
by local file existence checks, remote existence checks, metadata comparison,
and prompts. Re-running `encode` or `parse --upload` usually converged toward
the same state rather than blindly duplicating work.

The new workflow should make that property explicit instead of incidental:

- re-running a command should safely resume from the current local/remote state,
- already-applied metadata should be recognized as applied, not treated as an
  error,
- existing local files should be inspected and reused when valid,
- existing remote objects should be compared before upload or overwrite,
- destructive operations should remain explicit and safety-checked,
- repeated RSS/feed generation should not cause unnecessary metadata churn,
- prompts should describe the exact side effect that remains to be performed.

This is stricter than preserving the old behavior. The old behavior is the
compatibility baseline; the redesigned flow should make idempotency a first
class invariant.

## Original User Workflow

The original workflow was not plan-based. The user maintained `podspec.yaml`
directly and used command execution as the operational boundary.

1. Add or edit an episode in `podspec.yaml`.
2. Ensure the edited master exists locally under `config.localStorageDir`.
3. Run `mkpod encode <uid>` or `mkpod encode --all`.
4. Answer prompts for re-encoding, metadata save, output media upload, and any
   optional destructive remote master removal.
5. Run `mkpod parse --upload` when the feed should be generated and published.
6. Answer prompts for `lastBuildDate`, `podspec.yaml` rewrite, referenced image
   sync, RSS diff/upload, and feed upload.

The important property: encoding and feed publishing were separate command
invocations, but `encode` was not local-only. It interacted with S3 for episode
inputs, images, output media, and optional remote master deletion.

## `mkpod encode` Entry Point

Command shape:

```sh
mkpod encode [episode-uids...] | --all
mkpod encode --spec ./podcast/podspec.yaml 34
mkpod encode --spec ./podcast/podspec.yaml --all
mkpod encode --spec ./podcast/podspec.yaml --all --force
mkpod encode --spec ./podcast/podspec.yaml 34 --remove-remote-master
```

Flags:

- `--spec, -s`: YAML spec file, default `podspec.yaml`.
- `--all, -a`: process every episode, but skip episodes with existing local
  output unless `--force` is also set.
- `--force, -f`: answer prompts yes and force re-encode for `--all`.
- `--remove-remote-master, -R`: opt in to remote input-master deletion checks
  after encoding. This is the explicit destructive boundary for deleting remote
  masters.

Current prompt semantics:

- `prompt.New(dryrun=false, force=false)` asks on a TTY.
- Default answer in the survey prompt is `Yes`.
- If stdout is not a terminal, prompts answer `No`.
- `--force` answers `Yes`.

## `mkpod encode` Detailed Flow

`runEncodeWorkflow` performs the following steps.

1. Validate command arguments.
   - If no UID args are given and `--all` is false, fail.

2. Load `podspec.yaml`.
   - Uses `spec.New(specFile).Load(ctx)`.
   - YAML is decoded into `model.Podcast`.
   - Top-level required fields are validated.
   - defaults are applied during load.

3. Create services.
   - `prompter := prompt.New(false, askNoQuestions)`.
   - `encoderService := encode.New(prompter)`.
   - `storageClient := s3store.New(atom, prompter)`, unless `LocalOnly` is set.
   - `LocalOnly` is a later workflow addition. It is not part of the original
     encode behavior.

4. Select episodes.
   - With `--all`, every episode index is selected.
   - With UID args, each UID is looked up via `atom.ContainsEpisode(uid)`.
   - If a requested UID does not exist, the encoder logs a warning and returns
     an empty result for that UID.

5. For each selected episode, apply encoding defaults.
   - `spec.ApplyEpisodeDefaultsForEncoding(atom, episode)` is called.
   - `episode.Author` is filled from podcast author when missing.
   - `episode.Image` is filled from `config.defaultPodImage` when missing.
   - Missing title/image errors stop encoding.

6. Ensure episode `pubDate`.
   - `ensureEpisodePubDate` sets a pubDate when missing.
   - If it changes the episode, encode marks metadata changed.

7. Repair existing output metadata if possible.
   - If `episode.Output` is already set, `repairOutputMetadata` inspects the
     local output file.
   - It can fill or repair metadata such as type, length, and duration.
   - If it changes the episode, encode marks metadata changed.

8. Decide whether to encode.
   - Empty output path means encode.
   - `--force` means encode.
   - Missing local output file means encode.
   - `--all` with existing local output means skip encode.
   - Otherwise prompt: `Re-encode <filename>?`.

9. Before actual encoding, prepare local assets.
   - `prepareEpisodeAssetsForEncode` is called only if encoding will run.
   - It tries to ensure these local files exist:
     - episode input master,
     - podcast/encoding cover image,
     - effective episode image.
   - It checks the configured input and output buckets as possible sources.

10. Detect input content type.
    - The input file path is `localStorageDir + episode.Input`.
    - Content type is detected from the local input file.

11. Choose encode mode.
    - Video input:
      - empty format / `video` / `mp4` -> MP4.
      - `audio` -> preferred format if `m4a`/`m4b`, otherwise MP3 via ffmpeg.
      - `mp3` -> MP3 via ffmpeg.
      - `m4a`/`m4b` -> ffmpeg audio.
    - Audio input:
      - empty format / `audio` -> preferred format if `m4a`/`m4b`, otherwise
        MP3 via lame.
      - `mp3` -> MP3 via lame.
      - `m4a`/`m4b` -> ffmpeg audio.

12. Run encoder.
    - MP4 uses ffmpeg.
    - M4A/M4B uses ffmpeg audio.
    - MP3 from audio uses lame.
    - MP3 from video uses ffmpeg piped into lame.
    - Encoders update episode fields:
      - `output`,
      - `type`,
      - `length`,
      - `duration`.
    - Audio metadata includes title, album, artist, genre, pubDate/year, track,
      link/comment, subtitle/description, language, cover art, and chapters.

13. Post-encode hook runs even when actual encoding was skipped.
    - It receives `wasEncoded`.
    - It handles optional remote master deletion.
    - It handles output media upload checks.

14. Save changed metadata.
    - If any episode metadata changed, prompt:
      `Podcast metadata changed, rewrite <specFile>?`
    - If yes, `spec.Store.Save` rewrites `podspec.yaml`.
    - Save updates `lastBuildDate` as part of `spec.Save`.

## Asset Preparation During Encode

`prepareEpisodeAssetsForEncode` is supposed to make local assets available
before encoding.

It processes:

- `episode.Input` as `episode <uid> input`,
- `atom.Encoding.Coverfront` as cover image,
- `spec.EffectiveEpisodeImage(atom, episode)` as episode image.

It uses buckets:

- `config.aws.buckets.input`,
- `config.aws.buckets.output`.

Current asset decision behavior:

1. Normalize the storage key.
2. Compute local path under `localStorageDir`.
3. If the local file exists, return `skip-asset`.
4. If local file is missing, check each configured bucket in order.
5. If remote exists, return `download-asset`.
6. If no remote exists, fail.

Important mismatch:

- `s3.Client.DownloadFile` has bidirectional behavior: if local exists and
  remote is missing, it prompts to upload local to the remote bucket.
- `prepareEpisodeAssetsForEncode` currently calls `DownloadFile` only when the
  local asset is missing and remote exists.
- Therefore the current wrapper prevents `DownloadFile` from performing its
  local-master upload behavior.
- This means current encode does not reliably upload a local input master when
  remote input master is missing, even though the lower-level storage method has
  code for that case.

This mismatch is part of the original/current behavior problem and must be
resolved deliberately before `mkpod apply` can be correct.

## Output Media Upload During Encode

After each selected episode, `postEncodeFunc` checks the encoded output.

Input:

- episode output key,
- local output path under `localStorageDir`,
- output bucket,
- `wasEncoded`.

Behavior:

1. If `episode.Output` is empty, do nothing.
2. If local output is missing, skip upload with reason `local output is missing`.
3. If remote check is skipped because there is no client, skip upload.
4. Check whether `episode.Output` exists in output bucket.
5. If remote output is missing:
   - operation kind is `upload-output`,
   - prompt: `Upload local file <output> to output bucket?`,
   - if yes, upload local output file to output bucket.
6. If remote output exists and `wasEncoded == true`:
   - operation kind is `upload-output`,
   - prompt: `Overwrite remote file <output> with newly encoded version?`,
   - if yes, upload local output file to output bucket.
7. If remote output exists and `wasEncoded == false`:
   - skip upload.

This output upload behavior is part of original encode. A new/apply workflow
that encodes locally but does not run this post-encode remote check is not
equivalent to original encode.

## Remote Master Deletion During Encode

Remote master deletion is not automatic. It is gated by `--remove-remote-master`
and safety checks.

When enabled:

1. Bucket is `config.aws.buckets.input`.
2. Key is normalized from `episode.Input`.
3. Local master path is under `localStorageDir`.
4. If local master cannot be statted for non-missing reasons, warn and skip.
5. Get remote input master info from input bucket.
6. Evaluate safety with `s3store.EvaluateRemoteMasterRemoval`.
7. Safety outcomes:
   - local missing -> skip,
   - remote missing -> skip,
   - local too small compared to remote -> skip,
   - allowed -> delete remote master.

Current implementation detail:

- The flag is the explicit opt-in boundary.
- The current code does not ask a second prompt after `--remove-remote-master`
  when safety allows deletion; it deletes and logs warnings on failure.
- This differs from the flag help text, which says there is a prompt unless
  force is given.

## `mkpod parse` / Feed Generation

Command shape:

```sh
mkpod parse
mkpod parse --spec ./podcast/podspec.yaml
mkpod parse --spec ./podcast/podspec.yaml --dry-run
mkpod parse --spec ./podcast/podspec.yaml --upload
```

Flags:

- `--spec, -s`: YAML spec file, default `podspec.yaml`.
- `--upload, -u`: upload RSS and referenced images to output bucket.
- `--force, -f`: answer prompts yes.
- `--dry-run, -n`: prompts answer no; RSS goes to stdout instead of file.

Detailed flow:

1. Reject positional args.
2. Load `podspec.yaml`.
3. Log whether it will generate only or generate/upload.
4. Compute feed path: `localStorageDir + atom`.
5. Create prompt service and RSS renderer.
6. Prompt:
   `Refresh lastBuildDate (will update <feedPath> and optionally <specFile>)?`
7. If yes:
   - set `atom.LastBuildDate` to now,
   - prompt: `Podcast metadata changed, rewrite <specFile>?`
   - if yes, save `podspec.yaml`.
8. If dry-run:
   - write RSS to stdout.
9. Otherwise:
   - write RSS XML to local feed path,
   - log success.
10. If `--upload` and not dry-run:
    - create S3 storage client,
    - sync referenced images,
    - calculate feed upload decision,
    - show diff between remote RSS and local RSS,
    - prompt: `Upload new <feedFile>?`,
    - if yes, upload RSS with content type `text/xml`.
11. If `--upload` and dry-run:
    - log that RSS would upload,
    - attempt dry-run image checks.

## Referenced Image Sync During Publish

`parse --upload` / `publish` syncs referenced images before RSS upload.

Referenced images are collected from:

- podcast image,
- encoding coverfront,
- each episode effective image.

Behavior per image:

1. Normalize image key.
2. Decide whether to skip, download, upload, or check.
3. If remote output image exists and local is missing, download it.
4. If local exists and remote is missing/different, prompt:
   `Upload <label> image <key> to S3?`
5. If user refuses an upload considered required, return an error.
6. Upload image with detected content type (`image/jpeg` or `image/png`).

## `s3.Client.DownloadFile` Semantics

This method is not a pure downloader.

When called for `bucket/key`:

1. Compute local path under `localStorageDir`.
2. Ensure local directory exists.
3. If local file is missing:
   - create local file,
   - download remote object into it.
4. If local file exists:
   - get remote object size.
   - If remote object is missing:
     - log that remote does not exist and local will be used,
     - prompt: `Upload <localPath> to s3://<bucket>/<key>?`,
     - if yes, upload local file using bucket storage class.
     - return.
   - If remote size equals local file size:
     - skip download.
   - Otherwise:
     - overwrite local file with remote download.

This means old behavior could be bidirectional at the storage-method level.
Any higher-level wrapper that avoids calling `DownloadFile` when local exists
also avoids this upload opportunity.

## Current `mkpod new` / `mkpod apply` Divergence

Current `mkpod new` writes a saved plan. That is new behavior and is not part
of the original workflow.

Current `mkpod apply <new.plan.json>` does this:

1. Decode saved workflow `"new"`.
2. Add the episode to `podspec.yaml`, or resume if identical episode metadata
   is already present.
3. Run encode with `LocalOnly: true`.
4. Run feed generation with `Upload: false`.
5. Print `Publish with: mkpod publish`.

This diverges from original encode in important ways:

- It skips remote input/master checks.
- It skips the lower-level upload opportunity for a local master missing
  remotely.
- It skips output media upload checks.
- It still generates local RSS.
- It can rewrite `podspec.yaml` once when adding the episode, again when encode
  repairs/fills metadata, and again when feed generation refreshes
  `lastBuildDate`.
- It generates RSS immediately after encode, so feed generation can happen
  before the explicit publish step and then again during `publish`.

Therefore current `mkpod apply <new.plan.json>` is not equivalent to the
original manual `podspec.yaml` + `encode` + `parse --upload` workflow.

## Original Workflow Invariants To Preserve

These are the behavior-level invariants that existed before `new/apply` tried
to compose the workflow.

1. Encoding is not local-only by default.
2. Local output media is compared with remote output media after encode.
3. Missing remote output media prompts for upload.
4. Re-encoded output prompts before overwriting remote output media.
5. Remote master deletion is opt-in and safety-checked.
6. RSS upload is separate from encode.
7. RSS upload shows a remote/local diff before upload.
8. Referenced images are synced as part of feed publish.
9. Prompt defaults are interactive yes, non-terminal no, force yes, dry-run no.
10. `podspec.yaml` is the durable metadata source after encode fills output,
    type, length, duration, and pubDate.
11. Commands should be idempotent: re-running a completed or partially completed
    command should converge, resume, or no-op safely instead of failing on work
    already performed.

## Open Questions For The New Workflow

These are not solved here; they are the points to decide before redesigning
`new/apply`.

1. Should `mkpod apply <new.plan.json>` upload input masters, output media, or
   both?
2. Should master upload be explicit and centralized instead of hidden inside
   `DownloadFile`?
3. Should `apply` generate local RSS at all, or should RSS generation be only
   `inspect` / `publish` / a separate local feed command?
4. Should `apply` write `podspec.yaml` once at the end instead of multiple
   times across add/encode/feed phases?
5. Should feed `lastBuildDate` be changed during local apply, or only during
   publish?
6. Should output media upload happen before or after `podspec.yaml` is saved
   with generated metadata?
7. What is the exact production boundary for `publish`: RSS only, RSS plus
   images, or all remote artifacts?
