CREATE TABLE tenants (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (BTRIM(name) <> ''),
    is_legacy BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX tenants_single_legacy_idx
ON tenants (is_legacy)
WHERE is_legacy = TRUE;

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL CHECK (BTRIM(password_hash) <> ''),
    email_verified_at TIMESTAMP WITH TIME ZONE,
    disabled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_normalized CHECK (email = LOWER(BTRIM(email))),
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE TABLE tenant_memberships (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

CREATE INDEX tenant_memberships_user_id_idx
ON tenant_memberships (user_id);

CREATE TABLE sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (OCTET_LENGTH(token_hash) = 32),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx
ON sessions (user_id);

CREATE INDEX sessions_expires_at_idx
ON sessions (expires_at);

CREATE TABLE account_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    token_hash BYTEA NOT NULL UNIQUE CHECK (OCTET_LENGTH(token_hash) = 32),
    target_email TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    consumed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX account_tokens_user_purpose_idx
ON account_tokens (user_id, purpose);

CREATE INDEX account_tokens_expires_at_idx
ON account_tokens (expires_at);

-- Existing deployments receive an unclaimed legacy workspace. It is created only
-- when operator data already exists, so a fresh installation does not accumulate an
-- unused legacy tenant. Claiming it requires the protected bootstrap flow.
INSERT INTO tenants (name, is_legacy)
SELECT 'Legacy workspace', TRUE
WHERE EXISTS (SELECT 1 FROM app_settings)
   OR EXISTS (SELECT 1 FROM companies)
   OR EXISTS (SELECT 1 FROM jobs)
   OR EXISTS (SELECT 1 FROM digest_deliveries);
