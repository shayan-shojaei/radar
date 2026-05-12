BINARY := radar
CMD    := ./cmd/radar
BIN    := bin/$(BINARY)

.PHONY: run build install test lint release clean

run:
	go run $(CMD) --url $(url)

build:
	go build -o $(BIN) $(CMD)

install: build
	cp $(BIN) /usr/local/bin/$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run

release:
	GOOS=linux  GOARCH=amd64  CGO_ENABLED=0 go build -o bin/$(BINARY)-linux-amd64  $(CMD)
	GOOS=darwin GOARCH=amd64  CGO_ENABLED=0 go build -o bin/$(BINARY)-darwin-amd64 $(CMD)
	GOOS=darwin GOARCH=arm64  CGO_ENABLED=0 go build -o bin/$(BINARY)-darwin-arm64 $(CMD)

clean:
	rm -rf bin/
