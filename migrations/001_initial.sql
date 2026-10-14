-- Run with a migration owner, never with the application login.
CREATE TABLE IF NOT EXISTS tenants (
    id text PRIMARY KEY,
    version integer NOT NULL,
    data jsonb NOT NULL,
    CHECK (data->>'id' = id),
    CHECK (data->>'status' IN ('provisioning','active','suspended','offboarding','archived'))
);
CREATE TABLE IF NOT EXISTS memberships (
    tenant_id text NOT NULL REFERENCES tenants(id),
    user_id text NOT NULL,
    data jsonb NOT NULL,
    PRIMARY KEY (tenant_id, user_id),
    CHECK (data->>'tenant_id' = tenant_id AND data->>'user_id' = user_id)
);
-- Typed JSON documents keep the extension points flexible while SQL enforces
-- tenant ownership, external identity uniqueness and clinical relationships.
CREATE TABLE IF NOT EXISTS resources (
    tenant_id text NOT NULL REFERENCES tenants(id),
    kind text NOT NULL,
    id text NOT NULL,
    data jsonb NOT NULL,
    patient_id text GENERATED ALWAYS AS (data->>'patient_id') STORED,
    patient_kind text GENERATED ALWAYS AS (CASE WHEN data->>'patient_id' IS NOT NULL THEN 'patients' END) STORED,
    facility_id text GENERATED ALWAYS AS (data->>'facility_id') STORED,
    facility_kind text GENERATED ALWAYS AS (CASE WHEN data->>'facility_id' IS NOT NULL THEN 'facilities' END) STORED,
    external_id text GENERATED ALWAYS AS (data->>'external_id') STORED,
    PRIMARY KEY (tenant_id, kind, id),
    FOREIGN KEY (tenant_id, patient_kind, patient_id) REFERENCES resources(tenant_id, kind, id),
    FOREIGN KEY (tenant_id, facility_kind, facility_id) REFERENCES resources(tenant_id, kind, id),
    CHECK (data->>'tenant_id' IS NOT NULL AND data->>'tenant_id' = tenant_id),
    CHECK (COALESCE(data->>'id', data->>'event_id', '') = id),
    CHECK (kind NOT IN ('patients','orders','appointments','workflows') OR facility_id IS NOT NULL),
    CHECK (kind NOT IN ('orders','appointments') OR patient_id IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS tenant_external_identity ON resources(tenant_id,kind,external_id) WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS tenant_job_state ON resources(tenant_id,(data->>'status'),(data->>'due_at')) WHERE kind = 'jobs';
CREATE INDEX IF NOT EXISTS tenant_outbox_state ON resources(tenant_id,(data->>'published')) WHERE kind = 'outbox';
ALTER TABLE resources ENABLE ROW LEVEL SECURITY;
ALTER TABLE resources FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON resources;
CREATE POLICY tenant_isolation ON resources
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), ''));
CREATE TABLE IF NOT EXISTS control_audit (
    id text PRIMARY KEY,
    tenant_id text NOT NULL,
    action text NOT NULL,
    actor text NOT NULL,
    at timestamptz NOT NULL DEFAULT now()
);
-- Deployment creates a LOGIN that inherits this role. No PHI is granted to PUBLIC.
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'healthcare_runtime') THEN
        CREATE ROLE healthcare_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;
GRANT USAGE ON SCHEMA public TO healthcare_runtime;
GRANT SELECT, INSERT, UPDATE ON tenants, memberships TO healthcare_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON resources TO healthcare_runtime;
GRANT INSERT ON control_audit TO healthcare_runtime;
