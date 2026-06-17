BIN := bin/server

.PHONY: build run tidy

build:
	go build -o $(BIN) ./cmd/server

run: build
	./$(BIN) -config config.yaml

tidy:
	go mod tidy
