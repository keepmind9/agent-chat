.PHONY: build test fmt clean

build:
	go build ./cmd/server && go build ./cmd/mcp

test:
	go test ./... -v

fmt:
	gofmt -w .

clean:
	rm -f server mcp *.db
