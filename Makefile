NAME = mkpod
MODULE = github.com/sa6mwa/mkpod
#VERSION = $(shell git describe --tags --abbrev=0 2>/dev/null || echo 0)
VERSION = v0.1.0
DESTDIR = /usr/local/bin
SRC = $(MODULE)/cmd/$(NAME)
GOOS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH = amd64
GO = CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go

.PHONY: all
all: clean build

.PHONY: clean
clean:
	rm -rf bin

.PHONY: build
build: test bin/$(NAME)

.PHONY: test
test:
	$(GO) test -cover ./...

bin:
	mkdir bin

bin/$(NAME): bin
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	$(GO) build -v -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/$(NAME) $(SRC)
	strip -s bin/$(NAME)
	if which upx > /dev/null ; then upx -9 bin/$(NAME) ; fi

go.mod:
	go mod init $(MODULE)
	go mod tidy

.PHONY: install
install:
	install bin/$(NAME) $(DESTDIR)/$(NAME)

.PHONY: release
release:
	cp bin/$(NAME) bin/$(NAME)-$(shell go env GOOS)-$(shell go env GOARCH)-$(VERSION)
	cd bin && sha256sum $(NAME)-$(shell go env GOOS)-$(shell go env GOARCH)-$(VERSION) > checksums.txt
