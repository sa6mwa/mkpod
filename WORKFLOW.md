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

## Proposed Apply Direction

This section records a proposed direction for the redesigned `mkpod apply`
workflow. It is not yet implemented.

Core idea: all validation and all yes/no decisions should happen up front when
running `apply`. Execution should then follow the planned decisions exactly.

`apply` should:

1. Load the saved plan.
2. Load current `podspec.yaml`.
3. Compare the saved plan with the current metadata state.
4. Treat already-applied identical metadata as a resumable/idempotent state.
5. Inspect local master state.
6. Inspect remote master state.
7. Inspect local encode-time image/artifact state.
8. Inspect remote encode-time image/artifact state.
9. Inspect local production audio state.
10. Inspect remote production audio state.
11. Decide whether encoding is needed.
12. Decide whether production audio upload or overwrite is needed.
13. Decide whether `podspec.yaml` needs to be written.
14. Decide whether `podcast.rss` needs to be regenerated.
15. Present the complete set of pending side effects before making changes,
    unless `--yes` / `--force` is used.

After that preflight, execution should be one pass over those decisions.

### `--yes` / `--force`

`apply --yes` or `apply --force` should answer yes to all non-destructive or
explicitly requested upgrades in the apply plan.

For apply, this means it can forcefully upgrade:

- remote master,
- production/rendered audio,
- local generated metadata,
- local RSS regeneration when applicable.

This should not blur destructive boundaries. Remote master deletion remains a
separate explicit operation unless the workflow later defines otherwise.

`--force` does not manufacture missing local inputs. If a forced operation needs
to upload a local master or production artifact and that local file does not
exist, apply must fail. In particular, `--force` should not download a remote
master and then treat that downloaded file as the local source for a forced
remote-master overwrite.

### Master Sync

Masters are special because they are the source artifacts used to encode
production audio.

Master sync rules:

1. If local master exists and remote master is missing:
   - prompt to upload local master,
   - upload automatically with `--yes` / `--force`.
2. If local master exists and remote master exists:
   - compare remote/local state up front,
   - if different, prompt before overwrite,
   - overwrite automatically with `--yes` / `--force`.
3. If local master is missing and remote master exists:
   - download remote master.
4. If both local and remote master are missing:
   - fail before mutating anything that depends on the master.
5. If `--yes` / `--force` is set and local master is missing:
   - fail instead of downloading, because force means "use my local source to
     upgrade remote", not "download remote and re-upload it".

This master behavior should be explicit in the workflow layer, not hidden as a
side effect of `DownloadFile`.

Master sync should run before encoding in plain apply and before exit in
`--just-master` / `--preflight` mode. A local master that does not exist
remotely should be uploaded regardless of whether the run is only applying the
master portion or continuing through encoding.

### `--just-master` / `--preflight`

`apply --just-master <plan.json>` should apply only the master-related pieces of
the workflow. `--preflight` may be an alias for this, or `--just-master` may be
an alias for `--preflight`; the naming is still open.

For a new episode plan, `--just-master` should:

1. Apply or resume the non-production episode metadata in `podspec.yaml`.
2. Sync the master according to the master sync rules.
3. Sync local encode-time images/artifacts needed to later encode.
4. Not encode production audio.
5. Not upload production audio.
6. Not regenerate `podcast.rss`.

The resulting `podspec.yaml` may contain the new episode metadata but should not
gain encoded-output metadata such as:

- `output`,
- `type`,
- `length`,
- `duration`,
- generated production-audio fields.

`apply --just-master --yes` or `apply --just-master --force` should overwrite
the remote master without asking when the preflight says overwrite is needed,
but only if the local master exists and passed validation.

The operation must be idempotently resumable. After `--just-master`, a later
plain `mkpod apply <plan.json>` should see that metadata is already applied and
masters are already synced, then continue with the remaining encode/output/RSS
steps.

### Encode-Time Images And Artifacts

Apply also needs to reason about images and other local artifacts needed for
encoding, such as cover art used for tags.

These are similar to master sync in that encode may need them locally, but they
are not production audio. Apply should therefore:

1. Ensure required encode-time local artifacts exist before encoding.
2. Download them from remote if local is missing and remote exists.
3. Fail before encoding if a required artifact exists neither locally nor
   remotely.
4. Avoid treating publish-time image refresh as part of apply unless the image
   is required locally to encode.

Uploading or refreshing RSS-referenced images in the output bucket is primarily
part of `publish`, together with RSS upload. Apply only needs the local artifact
availability required to perform encoding.

### Production Audio Sync

Production audio means the rendered/encoded podcast media such as `.m4a`,
`.m4b`, `.mp3`, or `.mp4`.

Production audio differs from masters:

- masters are needed locally to encode,
- production audio is needed remotely for streaming,
- local production audio is useful but not always required if remote production
  audio already exists and metadata can point to it.

Rules:

1. If encoding is needed, local master must exist first.
2. If local production audio exists and remote production audio is missing:
   - prompt to upload,
   - upload automatically with `--yes` / `--force`.
3. If local production audio exists and remote production audio exists:
   - compare up front,
   - prompt before overwrite if the apply plan says this run upgrades it,
   - overwrite automatically with `--yes` / `--force`.
4. If local production audio is missing but remote production audio exists:
   - do not download it just to regenerate RSS,
   - if `podspec.yaml` has the correct output metadata for RSS, remote audio can
     be treated as already publishable.
5. If production audio metadata is missing and local production audio is also
   missing:
   - if remote production audio exists and remote metadata is enough to repair
     `podspec.yaml`, repair metadata without downloading when possible,
   - otherwise download/sync the production audio locally only when needed to
     derive missing metadata,
   - otherwise encode from the master.

Only masters need local sync by default because only masters are required as
inputs for encoding.

### RSS Regeneration

RSS regeneration should be decided up front and performed at most once.

Rules:

1. `--just-master` should not regenerate `podcast.rss`.
2. Plain `apply` may regenerate local `podcast.rss` if episode metadata changed
   or encoded metadata was produced/repaired.
3. The prompt for RSS regeneration should happen during preflight.
4. If accepted, RSS should be regenerated once at the very end, after:
   - metadata application,
   - pubDate/default repairs,
   - encoding,
   - output metadata updates,
   - final `podspec.yaml` write.
5. Apply should not refresh `lastBuildDate`; that belongs to `publish`.
6. RSS upload remains outside `apply` unless a later design explicitly moves it.

This avoids the current behavior where `podspec.yaml` and `podcast.rss` can be
rewritten repeatedly during one apply.

### State Machine / Comparison Package

The idempotent workflow needs a central comparison and decision component rather
than scattered checks in command handlers.

That component should:

1. Read plan state, `podspec.yaml`, local filesystem state, and remote object
   state.
2. Classify the workflow state with explicit states such as not started,
   metadata applied, master synced, artifacts ready, encoded, production audio
   synced, RSS ready, and complete.
3. Decide whether each transition is safe.
4. Produce the complete list of pending operations and prompts.
5. Re-check critical local/remote facts immediately before mutating.
6. Fail deterministically if the execution-time facts no longer match the
   preflight decision.

This should keep invariants in one place. The command layer should not duplicate
the same remote/local safety rules in multiple branches.

### Renew Existing Episodes

Some workflows need to re-apply, repair, or re-encode episodes that already
exist in `podspec.yaml` and no longer have an original `new.plan.json`.

`mkpod renew` should create saved plans from existing episode metadata:

```sh
mkpod renew 34
mkpod renew all
mkpod inspect renew-34.plan.json
mkpod apply renew-34.plan.json
```

`renew` should not mutate by itself. It should only read `podspec.yaml`, build
one or more saved plans, and let `mkpod apply <plan.json>` execute them.

Command shape:

- `mkpod renew <uid>` creates a plan for one existing episode.
- `mkpod renew all` creates plans for all existing episodes.

Use the positional `all` form rather than `--all`, because the command already
takes a positional episode selector and `all` is the natural selector value.

The generated renew plan should use the same `internal/workflow` state machine
as new episode plans. It should support the same apply behavior:

- upfront validation and decisions,
- idempotent resume/no-op behavior,
- master sync,
- encode-time artifact sync,
- production audio repair/sync,
- optional encode,
- one final local RSS regeneration decision,
- no RSS upload from apply.

Conceptually, `renew <uid|all>` replaces the old habit of directly running
`mkpod encode <uid>` or `mkpod encode --all` when the desired operation is to
bring existing episode artifacts back into the expected state.
