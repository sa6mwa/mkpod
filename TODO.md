# mkpod Refactor TODO

This file is the working plan for finishing the refactor.

## Target Shape

- [ ] Keep the codebase simple: domain types plus a small set of concrete services with narrow interfaces at the seams that actually vary.
- [ ] Optimize for one supported backend now: local filesystem + external host tools (`ffmpeg`, `ffprobe`, `lame`) + AWS S3.
- [ ] Remove architecture that exists only to satisfy "hexagonal purity" when there is only one implementation and no meaningful substitution pressure.
- [ ] Preserve clean boundaries where they pay off:
  - [ ] configuration/spec loading
  - [ ] media tool execution
  - [ ] RSS rendering
  - [ ] remote storage operations
  - [ ] interactive prompting
- [ ] Make testing the primary quality gate: unit tests for rules, integration tests for command orchestration, e2e tests for S3 workflows.

## Current Repo Findings

- [x] Cobra-based CLI exists under `cmd/` and is the real entrypoint used by `main.go`.
- [x] The old `urfave/cli` implementation still exists under `cmd/mkpod/` and should be treated as dead legacy code.
- [x] The current `internal/app/ports` package is over-split into many one-method or AWS-shaped interfaces.
- [x] AWS-specific concepts leak into generic ports, especially storage class handling and remote file administration.
- [x] `encoder`, `uploader`, and `downloader` mix orchestration, shell command construction, metadata updates, and backend concerns.
- [x] `libfdk_aac` is still hard-coded in encoder command templates.
- [x] Tests pass today, but coverage is concentrated in a few areas and there are no tests for the current encoder/preprocessor/uploader command paths.
- [x] Existing e2e coverage is useful but shell-heavy, manually provisioned, and not yet a reliable regression suite.

## Phase 1: Remove Legacy And Simplify Structure

- [x] Delete the obsolete `cmd/mkpod/` tree after confirming nothing in the current build depends on it.
- [x] Remove `urfave/cli/v2` from `go.mod` once the legacy tree is gone.
- [x] Replace `internal/app/ports` with fewer, behavior-oriented interfaces owned by the consuming package instead of a global "ports" package.
- [ ] Collapse adapter naming into plainer package names where possible.
- [ ] Introduce a simpler package layout, roughly:
  - [x] `internal/spec` for YAML load/save/defaulting/validation
  - [ ] `internal/podcast` for domain rules around episodes/feed generation
  - [x] `internal/media` for shared tooling concerns around ffmpeg/ffprobe/lame execution
  - [x] `internal/storage/s3` for S3 operations
  - [ ] `internal/cli` for Cobra commands and command wiring
- [ ] Avoid generic request/response structs when a concrete method signature is clearer.
- [ ] Move AWS-only constants and types out of generic abstractions.

## Phase 2: Define Cleaner Interfaces

- [ ] Replace `ForConfiguring` with a concrete spec store API, for example:
  - [x] `Load(path string) (*podcast.Spec, error)`
  - [x] `Save(path string, spec *podcast.Spec) error`
- [x] Replace `ForParsing` with a renderer that writes to `io.Writer` and a small helper for writing files.
- [x] Replace `ForEncoding` with a concrete encoding service that returns a result struct rather than mutating via callback-heavy flow where possible.
- [ ] Replace `ForUploading`, `ForDownloading`, and `ForAdministeringRemoteFiles` with one S3 client interface used only where needed, for example:
  - [x] `UploadFile`
  - [x] `StatObject`
  - [x] `DeleteObject`
  - [x] `DiffTextObject`
- [ ] Keep `Asker` or `Prompter` as a tiny interface because it genuinely varies between interactive, force, and dry-run behavior.
- [ ] Move command-level orchestration out of storage/encoder adapters and into application services or Cobra command handlers.
- [x] Replace sentinel UID control values (`-1`, `-2`) with explicit options structs.

## Phase 3: Media Pipeline Cleanup

- [x] Replace `libfdk_aac` with ffmpeg’s native AAC encoder everywhere.
- [x] Decide and document default AAC settings for podcast use:
  - [x] audio codec: `aac`
  - [x] bitrate mode and target
  - [x] container defaults for `m4a` and `m4b`
- [x] Remove repo guidance that implies custom ffmpeg builds are required.
- [x] Default to host-provided `ffmpeg`, `ffprobe`, and `lame` from `PATH`.
- [x] Make explicit tool paths optional overrides in config, not required fields.
- [x] Add startup/tool validation that reports actionable errors when required binaries are missing.
- [x] Refactor command construction so shell strings are minimized or removed in favor of direct `exec.CommandContext` argument lists.
- [x] Clean up temporary ffmetadata files after AAC encoding completes.
- [x] Add tests around command generation and temp-file cleanup.

## Phase 4: Spec Model And Validation Cleanup

- [x] Rename the internal aggregate to `Podcast`.
- [x] Keep YAML compatibility where practical even if internal type names change.
- [x] Split pure validation/defaulting from file I/O.
- [x] Define a clear rule set for:
  - [x] required top-level fields
  - [x] required episode fields before encode
  - [x] required episode fields before RSS emission
  - [x] which fields are defaulted from top-level values
- [x] Centralize all defaulting logic instead of spreading it across spec, parser, and encoder.
- [x] Decide whether `parse` should fail on invalid episodes or render only valid episodes with warnings; document and test the policy.
- [x] Review path expansion helpers and remove `panic`-based behavior from tilde expansion.
- [x] Normalize path joining and base URL handling.

## Phase 5: Add `init` Command

- [x] Add `mkpod init <directory>` command.
- [x] Make the target directory argument mandatory.
- [x] Support `mkpod init .` and arbitrary paths like `mkpod init ~/podcast`.
- [ ] Create a predictable starter structure, for example:
  - [x] target directory
  - [x] `podspec.yaml`
  - [x] `artwork/`
  - [x] `masters/`
  - [x] `output/` or document that outputs are written in-place under local storage
- [x] Write a minimal but valid YAML template tailored to the current simplified config model.
- [x] Decide overwrite policy:
  - [x] fail if target exists and is non-empty
  - [ ] optional `--force` later if needed
- [x] Print next-step guidance after initialization.
- [x] Add tests for empty dir init, nested dir creation, existing file conflicts, and path expansion.

## Phase 6: Command UX Improvements

- [x] Audit all help text so it matches current behavior and simplified architecture.
- [x] Make dry-run behavior consistent across commands.
- [x] Ensure `force` means "do not prompt" everywhere.
- [x] Review `encode --all` semantics and replace implicit magic with explicit language.
- [x] Improve error messages for missing tools, invalid config, missing local files, and S3 object mismatches.
- [x] Add examples to help output for common workflows.
- [x] Decide whether preprocessing belongs as a long-term subcommand or should remain a thin utility wrapper.

## Phase 7: Storage Cleanup

- [ ] Move all S3-specific logic into one package.
- [x] Reduce duplication between uploader, downloader, and AWS handler session setup.
- [ ] Revisit AWS SDK choice:
  - [ ] either keep AWS SDK v1 for now and simplify around it
  - [ ] or migrate to v2 as a separate, deliberate task
- [x] Keep remote master deletion safety checks, but move them into a tested service with explicit policy.
- [x] Add RSS/image existence checks without scattering S3 knowledge across command code.
- [x] Decide whether storage class support is worth exposing in the main CLI today; if not, make it an internal detail.

## Phase 8: Testing Expansion

- [x] Add unit tests for spec validation and defaulting rules.
- [x] Add unit tests for episode selection and encode planning logic.
- [x] Add unit tests for tool discovery and missing-binary failures.
- [x] Add unit tests for `init` directory/template generation.
- [x] Add unit tests for S3 safety logic using mocked storage responses.
- [ ] Add integration tests for:
  - [x] `mkpod parse`
  - [x] `mkpod encode`
  - [x] `mkpod preprocess`
  - [x] `mkpod init`
- [x] Add integration tests that invoke the Cobra command tree directly instead of relying only on `go run .`.
- [x] Keep a small number of black-box CLI smoke tests for installed-binary behavior.
- [x] Add encoder integration tests that use tiny fixture media files and verify observable outputs.
- [x] Add RSS golden-file tests for representative podcast specs.

## Phase 9: E2E Test Strategy

- [x] Keep S3-backed e2e tests, but make them clearly optional and isolated from default local test runs.
- [x] Split e2e into:
  - [x] local integration tests that do not require AWS
  - [x] opt-in AWS acceptance tests
- [ ] Replace brittle shell-grep assertions with clearer structured assertions where practical.
- [x] Add a documented test fixture lifecycle for the AWS buckets.
- [x] Make e2e target names reflect intent: `acceptance-aws`, `acceptance-remove-master`, etc.
- [x] Ensure e2e tests can bootstrap a fresh test workspace via `mkpod init`.

## Phase 10: Documentation

- [x] Rewrite `README.md` to match the current Cobra CLI and simplified architecture.
- [x] Remove references to the legacy CLI and custom ffmpeg build requirement.
- [x] Document required host dependencies: `ffmpeg`, `ffprobe`, `lame`, AWS credentials/profile expectations.
- [x] Document the podspec format with a minimal example and one richer example.
- [x] Document the `init` workflow as the default starting point.
- [x] Add a short architecture note describing the simplified design and why the previous ports/adapters split was reduced.

## Existing Items To Carry Forward

- [x] Refactor `cmd/mkpod/` to a new top-level Cobra-based CLI.
- [x] Implement `--remove-remote-master`.
- [x] Prevent remote master deletion when the local master is missing.
- [x] Default top-level `pubDate` when absent.
- [x] Fill missing episode author from top-level author and fail when neither exists where required.
- [x] Default explicitness correctly in RSS output.
- [x] Validate required top-level fields before parse/write.
- [ ] Complete full e2e integration coverage.
- [x] Remove leftover temporary ffmetadata files.
- [x] Support natural-language date inputs like `now`, `today`, `yesterday`, `HH:MM`, and `HHMM` within the current `pubDate` parser.

## Proposed Execution Order

- [x] 1. Delete legacy CLI and dependency leftovers.
- [ ] 2. Simplify package boundaries and replace the current global ports package.
- [x] 3. Refactor media execution and switch AAC encoding.
- [x] 4. Centralize spec validation/defaulting.
- [x] 5. Implement `init`.
- [ ] 6. Expand unit and integration coverage around the new seams.
- [x] 7. Tighten AWS acceptance tests and refresh docs.
