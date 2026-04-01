ALTER TABLE companies
ADD COLUMN source TEXT NOT NULL DEFAULT '',
ADD COLUMN external_id TEXT NOT NULL DEFAULT '',
ADD COLUMN domain TEXT NOT NULL DEFAULT '';

UPDATE companies
SET logo_url = ''
WHERE logo_url IS NULL;

ALTER TABLE companies
ALTER COLUMN logo_url SET DEFAULT '',
ALTER COLUMN logo_url SET NOT NULL;

UPDATE companies
SET domain = website
WHERE website IS NOT NULL
  AND website <> '';

ALTER TABLE companies
DROP COLUMN website;
