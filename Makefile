GO := go
NAME := obsidian-vault-lint
BIN := bin/$(NAME)

# Where `make install` puts the binary. ~/.local/bin by default because that is
# what is on PATH — `go install` would land it in $(GOPATH)/bin (~/go/bin) with
# GOBIN unset, which is not on PATH, so the tool would be invisible. Override
# either half:  make install PREFIX=/usr/local  |  make install BINDIR=/some/dir
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: build test tidy clean install

build:
	@mkdir -p bin
	$(GO) build -o $(BIN) .

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf bin

## install — build straight into BINDIR so obsidian-vault-lint is on PATH from
##           any vault, not just one workspace's direnv.
install:
	@mkdir -p $(BINDIR)
	$(GO) build -o $(BINDIR)/$(NAME) .
	@echo "installed $(BINDIR)/$(NAME)"
