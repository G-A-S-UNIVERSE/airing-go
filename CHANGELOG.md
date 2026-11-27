# Changelog

All notable changes to this project will be documented in this file.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-11-27

### Added
- Strict read-only ZTE C320 Telnet CLI driver (`internal/olt/zte`) with command allowlist (`ValidateReadOnlyCommand`), ONU ID validation, optical Rx power parser, and `ErrReadOnlyMode` reboot lock
- Multi-OLT configuration and 60s TTL cache (`cmd/airing/olt_manager.go`) via `AIRING_OLTS_JSON`
- Read-only telemetry HTTP endpoints: `GET /olts/{key}/summary`, `GET /olts/{key}/onus`, `GET /olts/{key}/onus/{onuID}`

## [0.1.1] - 2026-11-27

### Added
- Generic `WebhookNotifier` (`pkg/notify/webhook.go`) with optional Bearer auth and `Multi` dispatcher (`pkg/notify/multi.go`)
- Configurable `AIRING_NOTIFY_WEBHOOK_URL` and `AIRING_NOTIFY_WEBHOOK_TOKEN` environment variables
- Enriched metadata forwarding (`host`, `event_id`, `status`, `zabbix_severity`, `flow_records`, LibreNMS device fields) in `processEvent`

## [0.1.0-go] - 2026-11-26

### Added
- Go-native 4-layer ISP automation scaffold (`cmd/airing`, `internal/olt`, `internal/zabbix`, `internal/librenms`, `internal/akvorado`, `pkg/notify`)
- `BaseOLTAdapter` interface, thread-safe adapter registry, and `zte_c320` / `generic_cli` adapter stubs
- Zabbix JSON-RPC 2.0 client (`GetActiveTriggers`, `AcknowledgeProblem`)
- LibreNMS REST client (`GetDevices`, `ListDevices`, `GetDevice`) + `LibreNMSClient` interface and `MockClient`
- Akvorado flow query client (`GetFlowSummary`, `QueryFlows`) + `AkvoradoClient` interface and `MockClient`
- Bounded non-blocking `EventBus` (`internal/eventbus`) with single-worker drain
- Zabbix HMAC-SHA256 webhook handler (`POST /webhooks/zabbix`)
- Parallel `Enricher` (`internal/enricher`) querying LibreNMS and Akvorado under a 5s hard timeout
- Deterministic `RuleEngine` (`internal/rules`) mapping `OLT_DOWN`, `HIGH_BW`, and `RESOLVED` events
- HTTP API server endpoints (`GET /health`, `POST /webhooks/zabbix`, `GET /olts`, `POST /olts/{key}/reboot/{onuID}`) with Bearer auth
- End-to-end webhook integration test and GitHub Actions CI (`go build`, `go vet`, `go test -race`, `golangci-lint`)
