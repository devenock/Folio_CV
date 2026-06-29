CREATE TABLE section_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    section_id   UUID NOT NULL REFERENCES sections(id) ON DELETE CASCADE,
    sort_order   INTEGER NOT NULL DEFAULT 0,

    -- Common fields (used across section types)
    title        TEXT,           -- job title, degree name, skill category, project name
    subtitle     TEXT,           -- company, institution, issuer
    location     TEXT,
    start_date   TEXT,           -- free text: "Jan 2022", "2020", etc
    end_date     TEXT,           -- free text or "Present"
    description  TEXT,           -- main body text
    url          TEXT,           -- project link, certificate URL

    -- Structured extras stored as JSONB for flexibility
    -- skills: {"items": ["Go", "PostgreSQL", "Docker"]}
    -- bullets: {"items": ["Built X", "Achieved Y"]}
    -- tags: {"items": ["React", "TypeScript"]}
    meta         JSONB,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_section_items_section_id ON section_items(section_id);
CREATE INDEX idx_section_items_sort_order ON section_items(section_id, sort_order);
