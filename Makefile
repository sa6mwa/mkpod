NAME = mkpod
MODULE = github.com/sa6mwa/mkpod
TAG_VERSION ?= $(shell git describe --tags --exact-match HEAD 2>/dev/null || echo 0.0.0)
VERSION ?= $(patsubst v%,%,$(TAG_VERSION))
DESTDIR = /usr/local/bin
SRC = $(MODULE)
GOOS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH = amd64
GO = CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go
GO_BUILD_FLAGS = -trimpath -buildvcs=true
GO_TEST_FLAGS = -buildvcs=true -cover
RELEASE_DIR = dist
RELEASE_TARGETS = linux/amd64 linux/arm64 linux/arm/7 freebsd/amd64 freebsd/arm64 freebsd/arm/7 darwin/amd64 darwin/arm64
RELEASE_PLATFORMS = linux-amd64 linux-arm64 linux-armhf freebsd-amd64 freebsd-arm64 freebsd-armhf darwin-amd64 darwin-arm64

.PHONY: all
all: clean test vulncheck build

.PHONY: clean
clean:
	rm -rf bin $(RELEASE_DIR)

.PHONY: build
build: bin/$(NAME) strip

.PHONY: vulncheck
vulncheck:
	go run -buildvcs=true golang.org/x/vuln/cmd/govulncheck@latest -show verbose ./...

.PHONY: strip
strip:
	strip -s bin/$(NAME)

.PHONY: test
test:
	$(GO) test $(GO_TEST_FLAGS) ./...

bin:
	mkdir bin

bin/$(NAME): bin
	$(GO) build -v $(GO_BUILD_FLAGS) -ldflags '-s -w' -o bin/$(NAME) $(SRC)

go.mod:
	go mod init $(MODULE)
	go mod tidy

.PHONY: install
install:
	install bin/$(NAME) $(DESTDIR)/$(NAME)

.PHONY: release
release:
	rm -rf $(RELEASE_DIR)
	mkdir -p $(RELEASE_DIR)
	set -eu; \
	for target in $(RELEASE_TARGETS); do \
		GOOS_TARGET=$$(printf '%s' "$$target" | cut -d/ -f1); \
		GOARCH_TARGET=$$(printf '%s' "$$target" | cut -d/ -f2); \
		GOARM_TARGET=$$(printf '%s' "$$target" | cut -d/ -f3); \
		PLATFORM=$${GOOS_TARGET}-$${GOARCH_TARGET}; \
		if [ "$$GOARCH_TARGET" = "arm" ]; then PLATFORM=$${GOOS_TARGET}-armhf; fi; \
		BASENAME=$(NAME)-$(VERSION)-$$PLATFORM; \
		STAGE=$(RELEASE_DIR)/$$BASENAME; \
		mkdir -p "$$STAGE/bin" "$$STAGE/share/$(NAME)"; \
		cp README.md LICENSE "$$STAGE/share/$(NAME)/"; \
		if [ "$$GOARCH_TARGET" = "arm" ]; then \
			CGO_ENABLED=0 GOOS=$$GOOS_TARGET GOARCH=$$GOARCH_TARGET GOARM=$$GOARM_TARGET go build $(GO_BUILD_FLAGS) -ldflags '-s -w' -o "$$STAGE/bin/$(NAME)" $(SRC); \
		else \
			CGO_ENABLED=0 GOOS=$$GOOS_TARGET GOARCH=$$GOARCH_TARGET go build $(GO_BUILD_FLAGS) -ldflags '-s -w' -o "$$STAGE/bin/$(NAME)" $(SRC); \
		fi; \
		(cd $(RELEASE_DIR) && zip -qr "$$BASENAME.zip" "$$BASENAME"); \
		rm -rf "$$STAGE"; \
	done

.PHONY: release-check
release-check: release
	set -eu; \
	TMP=$$(mktemp -d); \
	trap 'rm -rf "$$TMP"' EXIT; \
	for platform in $(RELEASE_PLATFORMS); do \
		BASENAME=$(NAME)-$(VERSION)-$$platform; \
		ZIP=$(RELEASE_DIR)/$$BASENAME.zip; \
		test -f "$$ZIP"; \
		unzip -q "$$ZIP" -d "$$TMP/$$platform"; \
		test -d "$$TMP/$$platform/$$BASENAME"; \
		test -f "$$TMP/$$platform/$$BASENAME/share/$(NAME)/README.md"; \
		test -f "$$TMP/$$platform/$$BASENAME/share/$(NAME)/LICENSE"; \
		test -f "$$TMP/$$platform/$$BASENAME/bin/$(NAME)"; \
	done; \
	if [ "$$(go env GOOS)-$$(go env GOARCH)" = "linux-amd64" ]; then \
		"$$TMP/linux-amd64/$(NAME)-$(VERSION)-linux-amd64/bin/$(NAME)" --help >/dev/null; \
		"$$TMP/linux-amd64/$(NAME)-$(VERSION)-linux-amd64/bin/$(NAME)" --version | grep -q .; \
		"$$TMP/linux-amd64/$(NAME)-$(VERSION)-linux-amd64/bin/$(NAME)" version | grep -q .; \
	fi
