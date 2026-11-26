# airing-go

> Vendor-neutral FTTH ISP platform — Go-native

**airing-go** unifies the four operational layers of an ISP NOC into a single binary:

| Layer | Tool | Role |
|-------|------|------|
| Event spine | Zabbix 8 | Triggers, CEP, root-cause, acknowledge |
| Enrichment | LibreNMS | Device inventory, port baseline |
| Traffic intel | Akvorado | NetFlow anomaly, saturation detection |
| Execution | OLT adapters | ONU management (ZTE, Huawei, C-DATA, FiberHome) |

External systems (CRM chatbox, Slack, PagerDuty) receive events through the
`Notifier` interface — airing-go itself never imports any concrete notifier.

## Status

🚧 **Early development** — API is not stable.

## Quick start

```bash
git clone https://github.com/G-A-S-UNIVERSE/airing-go.git
cd airing-go
cp .env.example .env
go run ./cmd/airing
```

## Related

- [airing](https://github.com/G-A-S-UNIVERSE/airing) — Python predecessor (v0.1.0 stable, OLT adapters only)

## License

MIT © G-A-S-UNIVERSE contributors
