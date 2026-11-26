# Contributing to airing-go

## Quick Start

```bash
git clone https://github.com/G-A-S-UNIVERSE/airing-go.git
cd airing-go
cp .env.example .env
go test ./... -v -race -count=1
```

## Adding an OLT Adapter

Implement `olt.BaseOLTAdapter` in `internal/olt/<vendor>/` and register it with `olt.Register(key, factory)` in `init()`.

## Pull Request Process

1. Fork the repository and create a feature branch.
2. Run `go build ./...`, `go vet ./...`, and `go test ./... -v -race -count=1`.
3. Ensure `gofmt -l .` returns no unformatted files.
4. Open a pull request against `main`.
