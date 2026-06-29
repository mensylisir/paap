ALTER TABLE service_templates
    ADD COLUMN IF NOT EXISTS provision_mode VARCHAR(20) DEFAULT 'managed';

ALTER TABLE service_templates
    ADD COLUMN IF NOT EXISTS runtime_spec TEXT;

UPDATE service_templates
SET provision_mode = 'managed'
WHERE provision_mode IS NULL OR provision_mode = '';

ALTER TABLE service_installations
    ADD COLUMN IF NOT EXISTS provision_mode VARCHAR(20) DEFAULT 'managed';

ALTER TABLE service_installations
    ADD COLUMN IF NOT EXISTS runtime_spec TEXT;

UPDATE service_installations
SET provision_mode = 'managed'
WHERE provision_mode IS NULL OR provision_mode = '';

DROP INDEX IF EXISTS idx_service_installation_env_type;

DELETE FROM service_installations
WHERE id IN (
    SELECT duplicate.id
    FROM service_installations AS duplicate
    JOIN (
        SELECT environment_id, service_type, provision_mode, MIN(id) AS keep_id
        FROM service_installations
        WHERE deleted_at IS NULL
        GROUP BY environment_id, service_type, provision_mode
        HAVING COUNT(*) > 1
    ) AS grouped
    ON duplicate.environment_id = grouped.environment_id
    AND duplicate.service_type = grouped.service_type
    AND duplicate.provision_mode = grouped.provision_mode
    AND duplicate.id <> grouped.keep_id
    WHERE duplicate.deleted_at IS NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_service_installation_env_type_mode
ON service_installations (environment_id, service_type, provision_mode);
