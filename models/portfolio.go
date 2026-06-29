package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Portfolio struct {
	ID          string
	UserID      string
	Slug        string
	FullName    string
	Headline    *string
	Summary     *string
	Email       *string
	Phone       *string
	Location    *string
	LinkedInURL *string
	GithubURL   *string
	WebsiteURL  *string
	AvatarURL   *string
	CareerType  string
	Theme       string
	BuilderMode string
	IsPublic    bool
	RawJSON     json.RawMessage
	CVFilename  *string
	CVParsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const portfolioColumns = `
	id, user_id, slug, full_name, headline, summary, email, phone, location,
	linkedin_url, github_url, website_url, avatar_url, career_type, theme,
	builder_mode, is_public, raw_json, cv_filename, cv_parsed_at, created_at, updated_at
`

func scanPortfolio(row pgx.Row) (*Portfolio, error) {
	p := &Portfolio{}
	err := row.Scan(
		&p.ID, &p.UserID, &p.Slug, &p.FullName, &p.Headline, &p.Summary, &p.Email,
		&p.Phone, &p.Location, &p.LinkedInURL, &p.GithubURL, &p.WebsiteURL,
		&p.AvatarURL, &p.CareerType, &p.Theme, &p.BuilderMode, &p.IsPublic, &p.RawJSON,
		&p.CVFilename, &p.CVParsedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// CreateSkeletonPortfolio inserts a placeholder portfolio row immediately
// after upload, before the Claude parse completes.
func CreateSkeletonPortfolio(ctx context.Context, pool *pgxpool.Pool, id, userID, slug, fullName, cvFilename string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO portfolios (id, user_id, slug, full_name, career_type, theme, cv_filename)
		VALUES ($1, $2, $3, $4, 'general', 'professional', $5)
	`, id, userID, slug, fullName, cvFilename)
	return err
}

// MarkPortfolioReprocessing flags an existing portfolio as being re-parsed
// ahead of a CV re-upload. Existing sections/items are left untouched —
// the new CV's data is merged in once parsing completes.
func MarkPortfolioReprocessing(ctx context.Context, pool *pgxpool.Pool, id, cvFilename string) error {
	_, err := pool.Exec(ctx, `
		UPDATE portfolios SET cv_filename = $1, cv_parsed_at = NULL, updated_at = NOW()
		WHERE id = $2
	`, cvFilename, id)
	return err
}

// CompletePortfolioParse fills in the parsed fields and marks cv_parsed_at,
// finalizing the slug once the full name is known.
func CompletePortfolioParse(ctx context.Context, pool *pgxpool.Pool, id, fullName, headline, summary string,
	email, phone, location, linkedInURL, githubURL, websiteURL *string,
	careerType, theme, slug string, rawJSON []byte) error {
	now := time.Now()
	_, err := pool.Exec(ctx, `
		UPDATE portfolios SET
			full_name = $1, headline = $2, summary = $3,
			email = $4, phone = $5, location = $6,
			linkedin_url = $7, github_url = $8, website_url = $9,
			career_type = $10, theme = $11, slug = $12,
			raw_json = $13, cv_parsed_at = $14, updated_at = $14
		WHERE id = $15
	`, fullName, headline, summary, email, phone, location,
		linkedInURL, githubURL, websiteURL, careerType, theme, slug,
		rawJSON, now, id)
	return err
}

// MarkPortfolioParseFailed records that the Claude parse failed for this portfolio.
func MarkPortfolioParseFailed(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx, `UPDATE portfolios SET full_name = 'Parse failed', updated_at = NOW() WHERE id = $1`, id)
	return err
}

func FindPortfolioByUserID(ctx context.Context, pool *pgxpool.Pool, userID string) (*Portfolio, error) {
	row := pool.QueryRow(ctx, `SELECT `+portfolioColumns+` FROM portfolios WHERE user_id = $1`, userID)
	return scanPortfolio(row)
}

func FindPortfolioBySlug(ctx context.Context, pool *pgxpool.Pool, slug string) (*Portfolio, error) {
	row := pool.QueryRow(ctx, `SELECT `+portfolioColumns+` FROM portfolios WHERE slug = $1`, slug)
	return scanPortfolio(row)
}

func FindPortfolioByID(ctx context.Context, pool *pgxpool.Pool, id string) (*Portfolio, error) {
	row := pool.QueryRow(ctx, `SELECT `+portfolioColumns+` FROM portfolios WHERE id = $1`, id)
	return scanPortfolio(row)
}

// UpdatePortfolioField updates a single allow-listed column on a portfolio.
func UpdatePortfolioField(ctx context.Context, pool *pgxpool.Pool, id, field, value string) error {
	allowed := map[string]bool{
		"full_name": true, "headline": true, "summary": true, "email": true,
		"phone": true, "location": true, "linkedin_url": true, "github_url": true,
		"website_url": true, "avatar_url": true,
	}
	if !allowed[field] {
		return pgx.ErrNoRows
	}
	_, err := pool.Exec(ctx, `UPDATE portfolios SET `+field+` = $1, updated_at = NOW() WHERE id = $2`, value, id)
	return err
}

func UpdateTheme(ctx context.Context, pool *pgxpool.Pool, id, theme string) error {
	_, err := pool.Exec(ctx, `UPDATE portfolios SET theme = $1, updated_at = NOW() WHERE id = $2`, theme, id)
	return err
}

// ToggleIsPublic flips a portfolio's public/private flag and returns the new value.
func ToggleIsPublic(ctx context.Context, pool *pgxpool.Pool, id string) (bool, error) {
	var isPublic bool
	err := pool.QueryRow(ctx, `
		UPDATE portfolios SET is_public = NOT is_public, updated_at = NOW()
		WHERE id = $1
		RETURNING is_public
	`, id).Scan(&isPublic)
	return isPublic, err
}

// ToggleBuilderMode flips a portfolio between the classic section/item
// editor and the canvas block builder, and returns the new mode. This is
// the early, minimal version of mode-switching ahead of the full
// conversion/rollout UX (seeding a sensible starter layout, "revert"
// safety, etc.) planned for a later phase.
func ToggleBuilderMode(ctx context.Context, pool *pgxpool.Pool, id string) (string, error) {
	var mode string
	err := pool.QueryRow(ctx, `
		UPDATE portfolios SET
			builder_mode = CASE WHEN builder_mode = 'classic' THEN 'canvas'::builder_mode ELSE 'classic'::builder_mode END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING builder_mode
	`, id).Scan(&mode)
	return mode, err
}
