-- Apply with a SQL Server migration identity. Grant the runtime identity only
-- SELECT, INSERT, UPDATE on this table; keep migration credentials separate.
IF OBJECT_ID('healthcare_billing', 'U') IS NULL
CREATE TABLE healthcare_billing (
    tenant_id nvarchar(64) NOT NULL,
    id nvarchar(128) NOT NULL,
    order_id nvarchar(128) NOT NULL,
    amount_cents bigint NOT NULL CHECK(amount_cents >= 0),
    currency char(3) NOT NULL,
    status nvarchar(32) NOT NULL,
    PRIMARY KEY(tenant_id,id),
    CHECK(status IN ('draft','submitted','paid','rejected'))
);
