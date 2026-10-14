# Health-Care

Multi-tenant healthcare integration and workflow platform in Go, implemented from [Requirment.txt](Requirment.txt). Includes a REST API, optional gRPC reads, durable workflows and scheduling, PostgreSQL tenant isolation, and Kafka outbox/inbox processing.

## Run locally

Requires Go 1.26 or later (the Go launcher downloads the configured toolchain when permitted). In PowerShell:

```powershell
.\scripts\dev.ps1
```

This creates a random signing key in the ignored `.env` file and starts the API at `http://127.0.0.1:18080`. Development mode uses an ephemeral memory store and seeds two organizations: `north-health` / `north-admin` and `south-health` / `south-admin`. Memory mode loses records on restart; use PostgreSQL for durable or multi-process execution.

In another terminal, exercise the complete synthetic therapy workflow:

```powershell
.\scripts\smoke.ps1
```

Get a tenant token and inspect membership:

```powershell
$taskToken = go run ./cmd/token -subject north-admin -tenant north-health
$taskHeaders = @{ Authorization = "Bearer $taskToken" }
Invoke-RestMethod http://127.0.0.1:18080/api/v1/me -Headers $taskHeaders
```

Token generation is an offline operator tool, not an HTTP endpoint. Tokens expire in one hour by default. RS256 mode accepts tokens from the configured identity provider instead.

## Durable development environment

```powershell
.\scripts\dev.ps1 -ConfigureOnly
docker compose up --build -d
.\scripts\smoke.ps1
```

Compose runs PostgreSQL, migrations, the API and an embedded worker. Localhost ports are 18080 (HTTP), 9090 (gRPC) and 55432 (PostgreSQL). Set `HTTP_ADDR` for a different native port, or `HTTP_PORT` for Compose, and pass the matching `-BaseUrl` to the smoke script. The database credentials in Compose are for local development. Initialization creates a separate application login without table ownership or RLS bypass privileges.

Enable Kafka-compatible Redpanda and the outbox worker:

```powershell
docker compose --profile events up --build -d
```

Optional adapter services: `docker compose --profile databases up -d mongodb redis`. Stop containers with `docker compose --profile events --profile databases stop`; named volumes preserve records.

For a native API using Compose PostgreSQL, set `STORE=postgres` and `DATABASE_URL=postgres://healthcare_app:local-app-password@127.0.0.1:55432/healthcare?sslmode=disable` in `.env`. Configure a separate migration-owner `MIGRATION_DATABASE_URL` and run `go run ./cmd/migrate`. If your container runtime does not forward localhost ports, use the Compose API and containerized tests.

## Implemented behavior

| Area | Behavior |
| --- | --- |
| Tenant administration | Idempotent shared-schema provisioning, memberships, facility scopes, versioned quotas, suspension/reactivation/offboarding/archive states. |
| Authentication | HS256 or pinned RS256 JWTs; required issuer, audience, expiry and subject; current memberships; no implicit platform-admin clinical access. |
| Data isolation | Explicit tenant predicates, forced PostgreSQL RLS, transaction-local tenant settings, composite relationships and external identity uniqueness. |
| Clinical records | Facilities, patients, staff qualifications, rooms/chairs, therapy orders and atomic appointment reservations. |
| Workflow | Version-pinned infusion state machine, explicit insurance/inventory/clearance outcomes, checkpoints, cancellation and reservation compensation. |
| Jobs and schedules | Bounded workers, tenant fairness and quotas, memory admission, leases/fencing/recovery, exponential retries, dead-letter states, cron/interval/timezones, pause/resume/manual dispatch. |
| Messaging | Transactional audit/outbox, sequenced tenant events, acknowledged Kafka delivery, idempotent inbox outcomes, explicit offset commits and DLQ routing. |
| Interoperability | REST; authenticated gRPC patient/workflow reads; read-only FHIR Patient/Bundle subset. |
| Database adapters | PostgreSQL primary store; reusable SQL Server billing, MongoDB metadata and tenant-scoped Redis TTL cache repositories. |
| Resilience | Tenant-specific integration circuits, safe HTTP retries, bounded requests/responses, context cancellation and graceful shutdown. |
| Operations | JSON logs, clinical read/mutation audits, aggregate Prometheus metrics, health probes, non-root Docker image, Kubernetes HPA/PDB examples, CI and integration tests. |

Insurance, medication readiness and clinical clearance are explicitly recorded outcomes; transport success does not imply approval. Staff qualification records represent authorized input rather than independent credential verification. Billing records submission state and does not submit a real payer claim. Optional database repositories are integration components; the API does not automatically synchronize them with PostgreSQL.

## API

Send `Authorization: Bearer <token>`. `X-Tenant-ID` selects an authorized membership; a tenant-bound token cannot switch organizations. Tenant record creation and workflow/scheduler/job actions require `Idempotency-Key` (1–128 characters). Repeating the same input/key returns the original result; changed input returns HTTP 409.

| Method and path | Purpose |
| --- | --- |
| `GET /healthz`, `/readyz`, `/metrics` | Health, storage readiness and aggregate metrics. |
| `GET /api/v1/me` | Tenant, user, roles and facility scopes. |
| `POST /api/v1/platform/tenants` | Create tenant and initial administrator; platform operator only. |
| `GET /api/v1/platform/tenants[/{id}]` | Read tenant registry; platform operator only. |
| `PATCH /api/v1/platform/tenants/{id}` | Update lifecycle/quotas using current `version`. |
| `PUT /api/v1/memberships/{user}` | Tenant administrator manages roles, facilities and active state. |
| `POST /api/v1/facilities`, `/staff`, `/care-resources` | Configure scheduling resources. |
| `POST /api/v1/patients`, `/orders` | Create patient and therapy workflow. |
| `POST /api/v1/workflows/{id}/outcomes` | Record `{ "step": "insurance", "outcome": "approved" }`, or another supported outcome. |
| `POST /api/v1/workflows/{id}/cancel` | Cancel eligible workflow and release reservations. |
| `POST /api/v1/appointments` | Reserve a qualified nurse and chair/room after prerequisites pass. |
| `POST /api/v1/schedules` | Create `{ "kind": "reconciliation", "expression": "@every 15m" }`. |
| `POST /api/v1/schedules/{id}/actions` | `pause`, `resume`, or `trigger`. |
| `POST /api/v1/jobs/{id}/actions` | `cancel` or `retry` dead/cancelled jobs. |
| `GET /api/v1/{collection}[/{id}]` | Read permitted facilities, staff, care-resources, patients, orders, workflows, appointments, jobs, schedules, audit or events. |
| `GET /fhir/Patient[/{id}]` | Read-only Patient or search Bundle subset. |

Lists use `?limit=50&after=<next_cursor>`, maximum 100. Follow `next_cursor` even when facility filtering gives an empty page. Errors use `{ "error": "...", "correlation_id": "..." }` with HTTP 400/401/403/404/409/429 as appropriate.

Workflow outcomes: `insurance=approved|rejected`, `inventory=available|unavailable`, `clearance=approved|rejected`, `administration=completed`, `billing=submitted`. Later steps require the corresponding workflow state.

Set `GRPC_ADDR` to enable the read service defined in [api/healthcare.proto](api/healthcare.proto). Metadata uses `authorization` and optional `x-tenant-id`; authorization, facility scopes and tenant request quotas are shared with REST.

## Workers and events

The API embeds a worker by default. For separate processes, use PostgreSQL, set `WORKER_ENABLED=false` on the API, and run `go run ./cmd/workflow-worker`. Scheduling is part of the worker's transactional claim loop, which prevents duplicate dispatch across replicas.

Run `go run ./cmd/integration-worker` with `KAFKA_BROKERS` and `KAFKA_TOPIC` to deliver persisted outbox events. Without it, events remain unpublished. Delivery is at least once; event IDs and tenant sequence numbers support deduplication and ordering. Later events wait while an earlier tenant delivery lease is outstanding.

For inbound outcomes, configure `KAFKA_INPUT_TOPIC`, `KAFKA_DLQ_TOPIC`, `KAFKA_GROUP_ID` and `KAFKA_SERVICE_USER`. Register that service identity as an active `integration` membership with facility scope in every permitted tenant. Restrict topic ACLs to trusted producers/consumers: a shared-topic ACL does not isolate individual records.

```json
{
  "tenant_id": "north-health",
  "event_id": "unique-producer-event-id",
  "event_type": "workflow.outcome",
  "schema_version": 1,
  "workflow_id": "workflow-id",
  "step": "insurance",
  "outcome": "approved",
  "correlation_id": "opaque-correlation-id"
}
```

Offsets commit only after the outcome/inbox transaction succeeds or DLQ delivery is acknowledged. Transient errors receive bounded retries; malformed, unauthorized and exhausted events go to the protected DLQ. Replay is deduplicated by tenant/event ID.

## Verify

```powershell
go test ./...
go vet ./...
go build ./cmd/...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
docker compose --profile test run --rm test
```

Containerized tests include the race detector and verify real PostgreSQL RLS, relational constraints, rollback, durable state and competing worker connection pools. They add uniquely named synthetic test tenants. Native PostgreSQL tests require both `PG_TEST_ADMIN_URL` and `PG_TEST_RUNTIME_URL`; otherwise they skip. Native `go test -race ./...` requires a suitable C toolchain.

Other tests cover full workflows, approval gates, tenant/facility denial, platform-role separation, membership revocation, spoofed selectors, quota rollback, duplicate requests, simultaneous bookings, cancellation, suspension, stale leases, cron, gRPC, inbox replay, outbox recovery, JWT algorithms, memory reservations, cache keys and safe retries.

## Deployment scope

[deployments/kubernetes/api.yaml](deployments/kubernetes/api.yaml) expects an existing database, TLS ingress/proxy and `healthcare-runtime` Secret (`database-url`, `auth-key`). Publish your own image, apply migrations under a separate owner, and configure network policies, backups, region placement, key rotation and identity-provider settings. No cloud infrastructure is provisioned automatically.

The executable implements the shared-schema core. Additional requirement areas include dedicated tenant tiers, automated retention/deletion/restores, general configurable DAGs, holiday/home-travel scheduling, real payer/pharmacy/EHR contracts, HL7 v2, CDC, adaptive concurrency, callback plugins, mTLS and full observability/FHIR conformance. Their extension boundaries are described in [docs/architecture.md](docs/architecture.md). This is a reference implementation, not a claim of production operation or compliance certification.
