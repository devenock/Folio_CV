CREATE TYPE career_type AS ENUM (
    'developer', 'designer', 'creative', 'corporate',
    'academic', 'healthcare', 'education', 'hospitality',
    'legal', 'finance', 'marketing', 'general'
);

CREATE TYPE portfolio_theme AS ENUM (
    'professional', 'creative', 'minimal'
);

CREATE TABLE portfolios (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slug            TEXT UNIQUE NOT NULL,
    full_name       TEXT NOT NULL,
    headline        TEXT,
    summary         TEXT,
    email           TEXT,
    phone           TEXT,
    location        TEXT,
    linkedin_url    TEXT,
    github_url      TEXT,
    website_url     TEXT,
    avatar_url      TEXT,
    career_type     career_type NOT NULL DEFAULT 'general',
    theme           portfolio_theme NOT NULL DEFAULT 'professional',
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    raw_json        JSONB,
    cv_filename     TEXT,
    cv_parsed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_portfolios_user_id ON portfolios(user_id);
CREATE INDEX idx_portfolios_slug ON portfolios(slug);
