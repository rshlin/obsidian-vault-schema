GO := go
BIN := bin/obsidian-vault-lint

.PHONY: build test tidy clean

build:
	@mkdir -p bin
	$(GO) build -o $(BIN) .

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf bin
