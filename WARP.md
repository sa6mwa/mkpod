# mkpod context engineering

This branch (hexagonal) is a refactoring of mkpod (cmd/mkpod/mkpod.go
and other .go files under cmd/mkpod/) to the Ports and Adapters
Pattern (hexagonal architecture). The cmd/mkpod/ is the old version
and the top-level main.go holds the new refactored version using
spf13/cobra (cobra-cli).

## Structure

* internal/app/ports/ has the ports (all prefixed with For indicating what the port interface is for)
* internal/app/model holds the data-model and DTOs shared across packages
* internal/adapters/ contains all adapters (concrete types implementing ports interfaces) in individual packages
* scripts/ has scripts for building ffmpeg and a Blender addon to export markers
* podspec.yaml is an example podspec
* Makefile is how the CLI is built, tested, installed and packaged for release
* TODO.org is an Emacs org mode TODO list
* README.md describes the old usage unless updated to reflect the new usage

## Intent

The main intent is to fully migrate the functionality of the old mkpod
(cmd/mkpod/) to the new hexagonal architecture mkpod CLI under the
top-level directory and be able to run `go install
github.com/sa6mwa/mkpod@latest` to build and install it (the old
version used github.com/sa6mwa/mkpod/cmd/mkpod as path to the main app
which should be deprecated).

## Instructions

You are to consistently refactor the old mkpod (cmd/mkpod/) into the
new ports and adapters pattern mkpod cobra-based CLI under the
top-level directory. Features and functionality should be preserved,
the new mkpod should support everything the old mkpod did. Both ports
and adapters likely already exist for you to use, unless these need to
be refactored themselves. All new constructors in the adapters should
return the ports.For... interface (not a concrete type). The CLI app
using the adapters should never access the adapter concrete type or
any possible interface directly, it should only access the adapter
through a port interface (adapter constructors are an exception for
obvious reasons). Any data model required in an adapter should be
defined in the ports package instead, but structs returned from
adapters should genereally be avoided as these should probably be
interfaces instead (same here, ports.For... interfaces) with getter
methods to retrieve fields in non-exportable structs.

Never add packages directly to go.mod, always add them to the code
first, then let `go mod tidy` automatically retrieve the latest
versions of these packages and patch go.mod for you.

Always create unit tests for new code. Create mock adapters when
necessary. Most of the current codebase lack unit tests, add when
necessarry or when you modify existing codebase.
