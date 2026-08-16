-- 003_staff_role_scope.sql — role Admin Dinas dan hierarki scope unit.
ALTER TABLE units ADD COLUMN IF NOT EXISTS parent_id int REFERENCES units(id);
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('asn', 'verifikator_unit', 'verifikator_dinas', 'admin_dinas', 'pimpinan', 'admin'));
CREATE INDEX IF NOT EXISTS idx_units_parent ON units (parent_id);
COMMENT ON COLUMN units.parent_id IS 'Korwil parent untuk scope verifikator SMP/SKB';
