# Stockyard Brand (Standalone)

Compliance audit trail as a service. SHA-256 hash-chained ledger that any application can POST events to.

## What it does

Brand creates a tamper-evident log of everything that happens in your system. Each entry is cryptographically chained to the previous one. If anyone modifies or deletes a record, the chain breaks and verification fails.

Built for teams that need SOC2, HIPAA, or GDPR audit trails without building one from scratch.

## Features

- **Hash-chained ledger** — SHA-256 chain, every entry references the previous hash
- **Chain verification** — one API call proves the entire ledger is intact
- **Evidence export** — JSON bundles with chain proof for compliance audits
- **Policy templates** — SOC2, HIPAA, GDPR, EU AI Act preset rules
- **REST API** — POST events from any language, GET the audit trail
- **Single binary** — Go + embedded SQLite, no external dependencies
- **Self-hosted** — audit data never leaves your infrastructure

## Quick start

```bash
curl -fsSL https://stockyard.dev/brand/install.sh | sh
brand serve
```

## API

```bash
# Append an event
curl -X POST localhost:8750/api/events \
  -d '{"type":"user_login","actor":"alice","detail":{"ip":"1.2.3.4"}}'

# Verify chain integrity
curl localhost:8750/api/verify

# Export evidence pack
curl localhost:8750/api/evidence/export?from=2026-01-01&to=2026-03-31
```

## Pricing

- **Free:** 10,000 events, chain verification
- **Pro ($19/mo):** Unlimited events, evidence export, policy templates, webhooks

## Part of Stockyard

Brand is extracted from the Trust app in [Stockyard](https://stockyard.dev), the self-hosted LLM infrastructure platform.

## License

BSL 1.1
