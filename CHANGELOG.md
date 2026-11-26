# Changelog

All notable changes to this project will be documented in this file.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial Go-native scaffold (`cmd/airing`, `internal/olt`, `internal/zabbix`, `internal/librenms`, `internal/akvorado`, `pkg/notify`)
- `BaseOLTAdapter` interface and thread-safe adapter registry
- OpenAPI 3.0.3 draft contract in `api/openapi.yaml`
- Architecture documentation in `docs/ARCHITECTURE.md`
- GitHub Actions CI pipeline (`go build`, `go vet`, `go test -race`, `golangci-lint`)
