CREATE TABLE app_settings (
    id BIGINT PRIMARY KEY CHECK (id = 1),
    setup_complete BOOLEAN NOT NULL DEFAULT FALSE,
    role_keywords TEXT[] NOT NULL DEFAULT '{}',
    skill_keywords TEXT[] NOT NULL DEFAULT '{}',
    preferred_level_keywords TEXT[] NOT NULL DEFAULT '{}',
    penalty_level_keywords TEXT[] NOT NULL DEFAULT '{}',
    preferred_location_terms TEXT[] NOT NULL DEFAULT '{}',
    penalty_location_terms TEXT[] NOT NULL DEFAULT '{}',
    mismatch_keywords TEXT[] NOT NULL DEFAULT '{}',
    digest_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    digest_recipient TEXT NOT NULL DEFAULT '',
    digest_top_n INTEGER NOT NULL DEFAULT 10,
    digest_lookback_seconds BIGINT NOT NULL DEFAULT 86400,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
