CREATE TABLE companies (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    website TEXT,
    logo_url TEXT,
    created_at TIMESTAMP WITH TIMEZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIMEZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE jobs (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    location TEXT,
    url TEXT NOT NULL,
    source TEXT NOT NULL,
    posted_at TIMESTAMP WITH TIMEZONE,
    application_deadline TIMESTAMP WITH TIMEZONE,
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK(status IN ('active', 'archived')),
    created_at TIMESTAMP WITH TIMEZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIMEZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE jobs
ADD CONSTRAINT unique_source_url UNIQUE (source, url);