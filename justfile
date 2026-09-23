set dotenv-load := false

export GOCACHE := justfile_directory() / ".cache/go-build"
export GOMODCACHE := justfile_directory() / ".cache/go-mod"
export GOTOOLCHAIN := "local"
export GOLANGCI_LINT_CACHE := justfile_directory() / ".cache/golangci-lint"

default:
    @just --list

build:
    go build -o bin/kb ./cmd/kb

test:
    go test -race ./...

validate:
    go run ./cmd/kb validate

freshness:
    go run ./cmd/kb freshness

index:
    go run ./cmd/kb index

fmt-check:
    @sh scripts/check-format.sh

fmt:
    gofmt -w cmd internal modules

fix:
    go fix ./...

fix-check:
    @sh scripts/check-fix.sh

vet:
    go vet ./...

lint:
    golangci-lint run

check: fmt-check fix-check
    just vet
    just lint
    just test
    just validate
    just freshness
    just index
    go run ./cmd/kb search context --json
