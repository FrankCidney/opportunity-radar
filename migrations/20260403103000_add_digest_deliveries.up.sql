CREATE TABLE digest_deliveries (
    id BIGSERIAL PRIMARY KEY,
    recipient TEXT NOT NULL,
    digest_date DATE NOT NULL,
    job_count INTEGER NOT NULL DEFAULT 0,
    subject TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE digest_deliveries
ADD CONSTRAINT unique_recipient_digest_date UNIQUE (recipient, digest_date);
