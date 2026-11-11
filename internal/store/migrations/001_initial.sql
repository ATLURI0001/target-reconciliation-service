-- Version 1: intentionally serialized aggregate for the executable lab slice.
CREATE TABLE IF NOT EXISTS rp_schema_versions (
    version integer PRIMARY KEY,
    installed_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rp_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    document jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rp_audit (
    sequence bigint PRIMARY KEY,
    at timestamptz NOT NULL,
    actor text NOT NULL CHECK (actor <> ''),
    action text NOT NULL CHECK (action <> ''),
    resource text NOT NULL,
    result text NOT NULL
);
CREATE OR REPLACE FUNCTION rp_reject_audit_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit rows are append-only';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS rp_audit_no_update_delete ON rp_audit;
CREATE TRIGGER rp_audit_no_update_delete BEFORE UPDATE OR DELETE ON rp_audit
    FOR EACH ROW EXECUTE FUNCTION rp_reject_audit_mutation();
DROP TRIGGER IF EXISTS rp_audit_no_truncate ON rp_audit;
CREATE TRIGGER rp_audit_no_truncate BEFORE TRUNCATE ON rp_audit
    FOR EACH STATEMENT EXECUTE FUNCTION rp_reject_audit_mutation();
INSERT INTO rp_schema_versions(version) VALUES (1) ON CONFLICT DO NOTHING;
