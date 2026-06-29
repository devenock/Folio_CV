CREATE TYPE section_type AS ENUM (
    'experience', 'education', 'skills', 'projects',
    'certifications', 'awards', 'publications',
    'volunteer', 'languages', 'interests', 'custom'
);

CREATE TABLE sections (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    portfolio_id UUID NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    type         section_type NOT NULL,
    title        TEXT NOT NULL,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    is_visible   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sections_portfolio_id ON sections(portfolio_id);
CREATE INDEX idx_sections_sort_order ON sections(portfolio_id, sort_order);
