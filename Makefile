.PHONY: build test fmt clean

VERSION    ?= $(shell git describe --tags --exact-match 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS    := -s -w -X github.com/keepmind9/agent-chat/cmd.version=$(VERSION) \
              -X github.com/keepmind9/agent-chat/cmd.gitCommit=$(GIT_COMMIT) \
              -X github.com/keepmind9/agent-chat/cmd.buildTime=$(BUILD_TIME)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o agent-chat .

test:
	go test ./... -v

fmt:
	gofmt -w .

clean:
	rm -f agent-chat *.db *.db-journal
