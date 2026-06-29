CREATE TYPE block_type AS ENUM (
    'hero', 'heading', 'text', 'image', 'gallery', 'project_card',
    'experience_list', 'skill_list', 'education_list',
    'contact', 'social_links', 'divider', 'embed', 'button', 'spacer'
);

CREATE TABLE blocks (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    portfolio_id UUID NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    type         block_type NOT NULL,
    grid_x       INTEGER NOT NULL DEFAULT 0,
    grid_y       INTEGER NOT NULL DEFAULT 0,
    grid_w       INTEGER NOT NULL DEFAULT 12,
    grid_h       INTEGER NOT NULL DEFAULT 4,
    z_index      INTEGER NOT NULL DEFAULT 0,
    is_visible   BOOLEAN NOT NULL DEFAULT TRUE,
    content      JSONB NOT NULL DEFAULT '{}',
    style        JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_blocks_portfolio_id ON blocks(portfolio_id);
CREATE INDEX idx_blocks_portfolio_y ON blocks(portfolio_id, grid_y);
