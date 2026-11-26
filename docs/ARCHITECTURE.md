# airing-go Architecture

## Layer diagram

```
┌─────────────────────────────────────────────────┐
│                  airing-go binary               │
│                                                  │
│  OLT Adapters     Zabbix 8      LibreNMS  Akvorado│
│  (Execution)   (Event spine)  (Enrichment)(Traffic)│
│       │               │             │         │   │
│  ─────┴───────────────┴─────────────┴─────────┴──┤
│                  Core Event Bus                   │
│  ─────────────────────────┬───────────────────────┤
│                   Notifier interface              │
└───────────────────────────┼───────────────────────┘
                            │
              (private deployment code)
                 CRM chatbox · Slack · PagerDuty
```

## Component roles

| Component | Role | Interaction pattern |
|-----------|------|---------------------|
| OLT adapters | Ground truth — ONU status, Rx/Tx power, reboot | airing-go drives OLT |
| Zabbix 8 | Event spine — triggers, CEP, root-cause, acknowledge | Zabbix pushes to airing-go via webhook/MCP |
| LibreNMS | Enrichment — device inventory, port status, bandwidth baseline | airing-go polls LibreNMS |
| Akvorado | Traffic intelligence — NetFlow anomaly, top talker, saturation | airing-go queries Akvorado |
| Notifier | Event dispatch to external systems | airing-go calls Notify(); implementor is private |

## Why this split?

Zabbix 8 ships a native MCP server and Complex Event Processing engine —
it handles alert correlation so airing-go doesn't have to.

LibreNMS and Akvorado are read-only context sources: they enrich the picture
but do not drive actions.

OLT adapters are the only write path: airing-go can reboot an ONU, change
a VLAN, or push a config — with Zabbix as the trigger and LibreNMS/Akvorado
as the evidence.
