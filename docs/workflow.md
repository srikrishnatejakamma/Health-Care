# CareFlow workflow

The diagrams describe the implemented Go workflow and the portal actions that drive it. External outcomes are explicitly recorded by authorized people or trusted integrations; HTTP transport success does not imply clinical clearance, insurance approval or medication availability. The browser demo simulates transitions in memory; live mode commits through the API and its background worker.

## End-to-end therapy lifecycle

![Detailed CareFlow therapy workflow from tenant authorization through approvals, booking, cancellation, administration and billing](images/therapy-workflow.svg)

The editable diagram source is [therapy-workflow.mmd](diagrams/therapy-workflow.mmd). Positive outcomes may arrive in any order while approvals are pending; the three branches represent independent prerequisites, rather than automatically invoked payer or pharmacy services.

To regenerate the README graphic with Mermaid CLI:

```powershell
npx --yes --package @mermaid-js/mermaid-cli@12.0.0 mmdc -i docs/diagrams/therapy-workflow.mmd -o docs/images/therapy-workflow.svg -c docs/diagrams/mermaid-config.json -b white
```

Run this from the repository root. Mermaid CLI requires a compatible browser; when using an existing Chrome installation, supply its Puppeteer configuration with `-p <config.json>`.

| Current state | Authorized action / condition | Result |
| --- | --- | --- |
| New order | Patient belongs to an authorized facility; required order input passes validation | `awaiting_approvals`; order, workflow, initial job, audit and outbox changes commit together. |
| `awaiting_approvals` | Record `insurance=approved`, `inventory=available`, `clearance=approved` | Worker advances to `ready_to_schedule` once all three are recorded. |
| `awaiting_approvals` | Record insurance/clearance rejection or inventory unavailability | Worker advances to `rejected`. Missing outcomes keep the workflow pending. |
| `ready_to_schedule` | Book an eligible appointment with no conflicts and available capacity | Atomically reserve resources and enter `scheduled`. |
| `scheduled` | Cancel the confirmed appointment before administration | Release its reservation and return the workflow to `ready_to_schedule`. A new booking is a separate operation. |
| Eligible pre-administration state | Cancel the workflow | Release confirmed reservations and enter `cancelled`. Cancellation is denied after administration, in `awaiting_billing`, or after completion. |
| `scheduled` | Record `administration=completed` | Worker enters `awaiting_billing` and marks confirmed appointments completed. |
| `awaiting_billing` | Record `billing=submitted` | Worker enters `completed`. This records submission state; it does not send or settle a real payer claim. |

Invalid roles, facility scope, state transitions, duplicate external identities, stale versions and conflicting idempotency requests are rejected by the backend. Booking checks patient, nurse and resource overlaps, facility capacity, an active infusion-qualified nurse, matching facilities, and a future duration of at most 24 hours.

## Tenant authorization and transaction flow

```mermaid
flowchart TB
  Portal["React portal: authenticated live session"] --> Request["Same-origin API request<br/>Authorization + X-Tenant-ID"]
  Request --> JWT{"Signature, issuer, audience<br/>subject and expiry valid?"}
  JWT -->|No| Unauthorized["401: sign-in required"]
  JWT -->|Yes| Tenant{"Selected tenant authorized<br/>and currently active?"}
  Tenant -->|No| Denied["Reject tenant access"]
  Tenant -->|Yes| Membership["Load current active membership<br/>Resolve backend roles and facility scopes"]
  Membership --> Limits{"Role, scope and request budget allow operation?"}
  Limits -->|No| Refuse["Reject access or throttle request"]
  Limits -->|Yes| Operation{"Read or mutation?"}
  Operation -->|Read| Read["Apply tenant and facility predicates<br/>Audit clinical reads"]
  Operation -->|Mutation| Key{"Matching idempotency operation and key?"}
  Key -->|Same input already committed| Replay["Return original result"]
  Key -->|Different input| Conflict["409: idempotency conflict"]
  Key -->|New request| Transaction["Tenant-scoped transaction<br/>Validate business rules and quotas"]
  Transaction --> Commit["Commit domain changes + jobs<br/>audit + outbox + idempotency result"]
  Read --> Response["Authorized response to portal"]
  Replay --> Response
  Commit --> Response
  Response --> Cache["Cache only in current session memory"]
  Cache --> Change["Tenant / role / session change<br/>Abort requests, clear cache, remount page state"]
```

PostgreSQL adds forced row-level security and transaction-local tenant settings to application predicates. Memory storage is a development alternative. Current memberships and tenant lifecycle remain authoritative even if a token was issued earlier. Patient access is facility-scoped; the implementation does not provide a separate patient-specific ACL. Platform operator privileges do not imply clinical access.

Most domain mutations require `Idempotency-Key`. Tenant configuration instead uses version compare-and-swap; tenant administrators can reduce assigned quotas, while increases remain platform-control operations. An authorized session change clears client data; privacy masking is a display preference rather than authorization.

## Scheduler and background execution

```mermaid
flowchart TB
  Outcome["Recorded outcome / newly created workflow"] --> Queue["Durable tenant job queue"]
  Schedule["Due reconciliation schedule<br/>Cron or interval in tenant timezone"] --> Dispatch["Atomically enqueue execution<br/>Advance schedule next_run"]
  Dispatch --> Queue
  Queue --> Visit["Round-robin visit to active tenants"]
  Visit --> Budget{"Global worker slot and<br/>tenant memory budget available?"}
  Budget -->|No| Later["Leave work queued; revisit later"]
  Later --> Visit
  Budget -->|Yes| Claim["Check tenant concurrency quota<br/>Claim due job / recover expired lease"]
  Claim --> Fence["Persist unique lease ID<br/>Increment attempt and set lease expiry"]
  Fence --> Execute["Execute within bounded context<br/>workflow.advance or reconciliation"]
  Execute --> Result{"Execution succeeded?"}
  Result -->|Yes| Commit["Verify lease fencing and tenant state<br/>Commit workflow changes and completed job"]
  Result -->|No| Retry{"Retry attempts remain?"}
  Retry -->|Yes| Backoff["Persist queued retry<br/>Exponential backoff with jitter"]
  Backoff --> Queue
  Retry -->|No| Dead["dead<br/>Visible in job monitoring"]
  Commit --> Release["Release worker slot and memory reservation"]
  Backoff --> Release
  Dead --> Release
  Dead -.-> Manual["Authorized manual retry after review<br/>New idempotent job action"]
  Manual --> Queue
```

Claims and completion use transaction checks and lease fencing to prevent a stale worker from completing a newer attempt. Scheduling and job creation commit together; missed intervals coalesce. Pause/resume controls future schedule dispatch, while cancellation and retry apply to eligible job states. The portal shows persisted job history; adaptive scaling and arbitrary workflow publishing are not implemented.

## Outbox delivery and portal refresh

```mermaid
sequenceDiagram
  autonumber
  participant UI as React portal
  participant API as Go API
  participant DB as Tenant store
  participant Worker as Workflow worker
  participant Relay as Optional integration worker
  participant Kafka as Kafka broker
  UI->>API: Authorized mutation + tenant + idempotency key
  API->>DB: Commit domain change, job, audit and outbox
  DB-->>API: Transaction committed
  API-->>UI: Persisted mutation response
  Worker->>DB: Claim due tenant job with unique lease
  Worker->>DB: Fenced completion + workflow checkpoint + outbox
  opt Kafka integration worker configured
    Relay->>DB: Lease next eligible tenant outbox event
    Relay->>Kafka: Publish tenant-keyed event with stable event ID
    alt Broker acknowledges delivery
      Kafka-->>Relay: Acknowledgment
      Relay->>DB: Mark delivered using current lease
    else Delivery fails or acknowledgment is uncertain
      Relay->>DB: Preserve retryable delivery state
      Note over Relay,Kafka: Delivery is at least once, consumers deduplicate event IDs
    end
  end
  UI->>API: Open authenticated SSE connection
  loop Approximately every 15 seconds
    API->>DB: Recheck tenant lifecycle and membership
    Note over API: Revalidate token and close stream if access is invalid
    API-->>UI: Tenant refresh hint with no clinical payload
    UI->>API: Refetch authorized current-session queries
    API->>DB: Apply current role and facility scopes
    API-->>UI: Updated permitted records
  end
```

SSE emits periodic refresh hints rather than pushing Kafka events or clinical records into the browser. Collection polling remains available when streaming reconnects. Without an integration worker, outbox events remain undelivered. Inbound Kafka outcomes use a registered, scoped integration identity and tenant/event inbox deduplication; offsets commit after a successful application transaction or acknowledged dead-letter delivery.

## Portal actions and backend contracts

| Portal action | API / processing path |
| --- | --- |
| Connect workspace | `GET /api/v1/me` validates membership and returns role/facility scope. |
| Register or edit patient | `POST /api/v1/patients`; `PATCH /api/v1/patients/{id}`. Editing does not transfer facilities. |
| Create therapy | `POST /api/v1/orders` creates the version-pinned workflow. |
| Record prerequisite, administration or billing result | `POST /api/v1/workflows/{id}/outcomes`; worker evaluates the next transition. |
| Confirm appointment | `POST /api/v1/appointments` validates and reserves resources atomically. |
| Release appointment | `POST /api/v1/appointments/{id}/cancel`; a new reservation requires a separate booking. |
| Cancel therapy | `POST /api/v1/workflows/{id}/cancel` checks eligibility and compensates reservations. |
| Configure or dispatch schedules | `POST /api/v1/schedules`; `POST /api/v1/schedules/{id}/actions`. |
| Retry or cancel jobs | `POST /api/v1/jobs/{id}/actions`. |
| Monitor changes | `/api/v1/portal/overview`, `/operations`, `/audit`, `/events` and `/stream`, with their respective role restrictions. |

The visual designer saves tenant-specific drafts locally. Draft graphs do not replace the running v1 state machine. Real payer claims, inventory synchronization, external callback registries and provider-specific clinical integrations require additional backend services.
