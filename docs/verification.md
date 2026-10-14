# Verification record

The final shared-schema implementation was checked with Go 1.26.9 and the dependency versions recorded in `go.mod`/`go.sum`.

| Check | Result |
| --- | --- |
| `go test ./...` | Passed; optional native PostgreSQL test skips without its connection variables. |
| `go vet ./...` | Passed. |
| `go build ./cmd/...` | Passed for all five commands. |
| `docker compose --profile test run --rm test` | Passed with the race detector and real PostgreSQL 17. |
| `docker compose build api` | Passed; image contains API, workflow worker, integration worker, migration tool and token tool. |
| `scripts/smoke.ps1` against the patched native executable | Passed through approvals, appointment reservation, administration and billing completion. |
| `govulncheck@v1.9.0 ./...` | Zero reachable vulnerabilities; one advisory in required but uncalled module code was reported separately by the scanner. |
| YAML parsing and Compose configuration | Passed for OpenAPI, Compose, Kubernetes, Prometheus and CI artifacts. |

Real PostgreSQL tests cover forced RLS without a tenant setting, connection context cleanup, cross-tenant relationships, identifier uniqueness, rollback, persisted reads through another pool, and competing worker claims. API tests cover tenant/facility authorization, membership revocation, JWT algorithms, idempotency, quota rollback, booking conflicts, cancellation guards, suspension, lease recovery, workflow version pinning and clinical read auditing. Messaging tests use an injectable broker sink and in-process gRPC transport to check outbox recovery/order, inbox replay and protocol authorization.

The local container runtime did not forward PostgreSQL's port to Windows, so database checks ran within the Compose network. No real payer, EHR, pharmacy, Kafka, SQL Server, MongoDB or Redis environment was used for these checks. Those contracts and operational validations remain environment-dependent as described in `architecture.md`.
