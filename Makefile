.PHONY: build test fmt clean

build:
	go build -o server ./cmd/server
	go build -o mcp ./cmd/mcp

test:
	go test ./... -v

fmt:
	gofmt -w .

clean:
	rm -f server mcp *.db
