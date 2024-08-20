NAME = mkpod
MODULE = github.com/sa6mwa/mkpod
VERSION = $(shell git describe --tags --abbrev=0 2>/dev/null || echo 0)
DESTDIR = /usr/local/bin
SRC = $(MODULE)/cmd/$(NAME)
GOOS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH = amd64
GO = CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go

.PHONY: all
all: clean build

.PHONY: clean
clean:
	rm -f $(NAME)

.PHONY: build
build: test $(NAME)

.PHONY: test
test:
	$(GO) test -cover ./...

$(NAME):
	$(GO) build -v -ldflags '-s -X main.version=$(VERSION)' -o $(NAME) $(SRC)

go.mod:
	go mod init $(MODULE)
	go mod tidy

.PHONY: install
install: $(NAME)
	install $(NAME) $(DESTDIR)/$(NAME)
