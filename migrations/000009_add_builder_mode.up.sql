CREATE TYPE builder_mode AS ENUM ('classic', 'canvas');

ALTER TABLE portfolios ADD COLUMN builder_mode builder_mode NOT NULL DEFAULT 'classic';
