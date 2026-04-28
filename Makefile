.PHONY: build test fmt clean

build:
	go build -o agent-chat .

test:
	go test ./... -v

fmt:
	gofmt -w .

clean:
	rm -f agent-chat *.db *.db-journal
