# Architecture

`mkpod` used to experiment with a broad ports-and-adapters split. That added
more interface surface than the project actually needed, especially with only
one supported storage backend and a CLI-driven workflow. The current structure
keeps the boundaries that pay for themselves and removes the rest.

## Current shape

- `cmd/`: Cobra commands and CLI orchestration.
- `internal/spec`: YAML load/save, top-level validation, defaults, and shared
  episode rules used by encode and RSS rendering.
- `internal/rss`: RSS template rendering and RSS-specific helpers.
- `internal/media`: shared host-tool discovery and execution concerns.
- `internal/storage/s3`: concrete S3 upload, download, diff, and object admin
  operations.
- `internal/media/encode`, `internal/media/preprocess`, `internal/prompt`, `internal/logging`:
  concrete services around media processing, prompting, and logging.
  `preprocess` is intentionally just a small optional ffmpeg utility, not a core application layer.
- `internal/app/model`: the existing domain/data structures loaded from
  `podspec.yaml`.

## Design intent

- Prefer concrete services over global “ports” packages.
- Keep interfaces only where behavior genuinely varies, such as prompting.
- Optimize for one backend now: local files, host-provided media tools, and
  AWS S3.
- Keep command orchestration visible in the CLI layer rather than hiding it
  behind generic abstractions.
- Put validation and defaults into testable pure functions where possible.
- Keep required top-level fields and top-level-derived episode defaults explicit in `internal/spec`.
- AAC outputs use ffmpeg's built-in `aac` encoder with the configured `encoding.abr` target; storage-class handling remains an internal S3 concern rather than a CLI surface.

## Why this is simpler

- The CLI is the application boundary, so the code now follows the actual use
  case instead of modeling hypothetical adapters everywhere.
- S3 concerns live in one package instead of being split across uploader,
  downloader, and administrative “adapter” packages.
- RSS and spec rules are shared through explicit helper functions instead of
  duplicated field checks spread across commands and renderers.
- Encoder behavior now returns explicit results rather than requiring callers
  to inspect mutable service state.

## What is intentionally not abstracted yet

- Storage backends other than AWS S3.
- Alternative media toolchains beyond `ffmpeg`, `ffprobe`, and `lame`.
- The internal aggregate is now `Podcast`; the YAML key remains `atom` for compatibility.

If those become real product requirements later, the current seams are narrow
enough to evolve without reintroducing the earlier interface sprawl.
