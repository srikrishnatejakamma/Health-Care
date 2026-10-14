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

## CareFlow React portal

Checked locally on 2026-10-14 with Node.js 22.18.0, Go 1.26.9 and installed Chrome. Frontend dependency versions are recorded in `careflow-react-portal/package-lock.json`.

| Check | Result |
| --- | --- |
| `npm run build` | TypeScript and Vite production build passed. |
| `npm test` | 3 files, 9 unit/component tests passed. |
| Playwright demo suite | All 8 journeys passed: module rendering, tenant isolation, approval/booking/billing, role denial, masking/theme, mobile layout, designer drafts and automated accessibility. |
| Playwright live API journey | Passed against the local Go API: facility creation, patient registration/editing, outbox event rendering and no token/clinical record persistence in browser storage. |
| Tenant/role remount regression | Both targeted browser journeys passed after resetting page state on session changes. |
| Desktop/mobile review | Dashboard screenshots inspected at 1440px and 390px; no mobile horizontal overflow. |
| Automated accessibility | Light dashboard, dark patient list and dark dashboard scans passed; this does not certify full WCAG conformance. |
| Portal container | Clean `npm ci`, production image build, non-root Nginx configuration, read-only filesystem with writable temporary/config mounts, SPA fallback and security headers passed. |
| Local locked install | `npm ci --dry-run --ignore-scripts --no-fund --no-audit` passed. |
| Backend regressions | `go test ./...` and `go vet ./...` passed, including new portal authorization, reservation release, quota ceilings and authenticated SSE tests. |
| Deployment files | Compose configuration and YAML parsing of API, Kubernetes and CI definitions passed. |

The dependency lock was repaired after a clean container install exposed conflicting optional Ajv peers; clean installation was then verified in the Node 22 container. Vite emits harmless third-party Zod annotation warnings. OIDC login requires an identity provider with matching API JWT settings and was not exercised against an external provider. Kubernetes manifests and CI were checked locally but were not run in a cluster or GitHub Actions during this session. Real external inventory, payer claims, broker metrics, callback registries and arbitrary workflow publishing are explicitly identified as unavailable capabilities rather than simulated live integrations.
