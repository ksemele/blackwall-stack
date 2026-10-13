BIN_DIR := bin
PKG     := ./...

.PHONY: build install test lint build-linux clean

build:
	go build -o $(BIN_DIR)/blackwall ./cmd/blackwall

install:
	go install ./cmd/blackwall

test:
	go test -race $(PKG)

lint:
	golangci-lint run

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/blackwall-linux-amd64 ./cmd/blackwall

clean:
	rm -rf $(BIN_DIR)
