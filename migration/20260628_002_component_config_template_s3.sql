ALTER TABLE component_config_templates
  ADD COLUMN IF NOT EXISTS s3_bucket varchar(100) DEFAULT '',
  ADD COLUMN IF NOT EXISTS s3_key varchar(500) DEFAULT '';

