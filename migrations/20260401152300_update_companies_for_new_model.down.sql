ALTER TABLE companies
ADD COLUMN website TEXT;

UPDATE companies
SET website = NULLIF(domain, '');

ALTER TABLE companies
DROP COLUMN domain,
DROP COLUMN external_id,
DROP COLUMN source;

ALTER TABLE companies
ALTER COLUMN logo_url DROP NOT NULL,
ALTER COLUMN logo_url DROP DEFAULT;
