DROP TABLE IF EXISTS webhook_deliveries CASCADE;
DROP TABLE IF EXISTS refunds CASCADE;
ALTER TABLE tenants DROP COLUMN IF EXISTS webhook_url;
ALTER TABLE tenants DROP COLUMN IF EXISTS webhook_signing_secret;
