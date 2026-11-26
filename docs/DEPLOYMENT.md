# Deployment Guide

## 1. Environment Configuration

Copy `.env.example` and populate secrets and upstream URLs:

```bash
cp .env.example /etc/airing-go/airing.env
chmod 600 /etc/airing-go/airing.env
```

Required variables:

| Variable | Purpose |
|---|---|
| `ZABBIX_WEBHOOK_SECRET` | HMAC-SHA256 secret verifying `X-Zabbix-Signature` on `POST /webhooks/zabbix` |
| `AIRING_API_TOKEN` | Bearer token protecting `GET /olts` and `POST /olts/{key}/reboot/{onuID}` |
| `LIBRENMS_URL`, `LIBRENMS_API_TOKEN` | LibreNMS REST endpoint & `X-Auth-Token` |
| `AKVORADO_URL` | Akvorado flow query endpoint |
| `AIRING_LISTEN_ADDR` | HTTP bind address (default `:8090`) |

## 2. Linux Binary + systemd

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /usr/local/bin/airing-go ./cmd/airing
sudo useradd --system --no-create-home --shell /usr/sbin/nologin airing || true
sudo cp deploy/airing-go.service /etc/systemd/system/airing-go.service
sudo systemctl daemon-reload
sudo systemctl enable --now airing-go
sudo systemctl status airing-go
```

## 3. Docker

```bash
docker build -t airing-go:v0.1.0 .
docker run -d --name airing-go \
  --env-file /etc/airing-go/airing.env \
  -p 8090:8090 \
  --restart unless-stopped \
  airing-go:v0.1.0
```

## 4. Smoke Test

```bash
# Health check (no auth)
curl -sf http://127.0.0.1:8090/health

# Registered OLT adapters (Bearer auth)
curl -sf -H "Authorization: Bearer $AIRING_API_TOKEN" http://127.0.0.1:8090/olts

# Signed Zabbix webhook test
BODY='{"event_id":"smoke-1","event_name":"OLT_DOWN","host":"olt-region1-01","severity":"high","status":"PROBLEM","tags":{"olt_key":"zte_c320"},"timestamp":1700000000}'
SIG="sha256=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$ZABBIX_WEBHOOK_SECRET" | awk '{print $NF}')"
curl -sf -X POST http://127.0.0.1:8090/webhooks/zabbix \
  -H "Content-Type: application/json" \
  -H "X-Zabbix-Signature: $SIG" \
  -d "$BODY"
```
