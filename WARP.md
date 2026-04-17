# mkpod context engineering

This repository no longer follows the original broad ports-and-adapters experiment described in older planning notes. The current branch has been simplified to concrete services with narrow seams where variation actually matters.

## Current structure

* `cmd/` contains the Cobra CLI and command orchestration.
* `internal/app/model` contains the podspec-backed data model and XML helper structs.
* `internal/spec` handles podspec load/save, validation, defaults, and shared episode rules.
* `internal/rss` renders RSS output.
* `internal/media`, `internal/media/encode`, and `internal/media/preprocess` handle host media tooling and encoding/preprocessing flows.
* `internal/storage/s3` contains the concrete AWS S3 integration.
* `internal/prompt` and `internal/logging` contain prompting and logging helpers.
* `e2e/` contains opt-in AWS acceptance coverage.

## Intent

The goal is to keep `mkpod` simple and correct for one supported backend: local files, host-provided `ffmpeg`/`ffprobe`/`lame`, and AWS S3. Abstractions should be introduced only when there is real substitution pressure or testability value.

## Working rules

* Preserve podcast-publishing behavior first; refactors should not degrade the generated RSS or asset workflow.
* Prefer concrete services over new global interface packages.
* Keep command wiring visible in `cmd/` rather than hiding orchestration behind generic adapters.
* Add tests for every behavior change. Prefer observable CLI, RSS, and S3 behavior over implementation-detail assertions.
* Build the CLI as `bin/mkpod` under the repository root.
* Use host tools from `PATH` unless the spec explicitly overrides them.
