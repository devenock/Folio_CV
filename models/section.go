package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Section struct {
	ID          string
	PortfolioID string
	Type        string
	Title       string
	SortOrder   int
	IsVisible   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func FindSectionByID(ctx context.Context, pool *pgxpool.Pool, id string) (*Section, error) {
	s := &Section{}
	err := pool.QueryRow(ctx, `
		SELECT id, portfolio_id, type, title, sort_order, is_visible, created_at, updated_at
		FROM sections WHERE id = $1
	`, id).Scan(&s.ID, &s.PortfolioID, &s.Type, &s.Title, &s.SortOrder, &s.IsVisible, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

func CreateSection(ctx context.Context, pool *pgxpool.Pool, portfolioID, sectionType, title string, order int) (*Section, error) {
	s := &Section{}
	err := pool.QueryRow(ctx, `
		INSERT INTO sections (portfolio_id, type, title, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING id, portfolio_id, type, title, sort_order, is_visible, created_at, updated_at
	`, portfolioID, sectionType, title, order).Scan(
		&s.ID, &s.PortfolioID, &s.Type, &s.Title, &s.SortOrder, &s.IsVisible, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ListSectionsByPortfolio(ctx context.Context, pool *pgxpool.Pool, portfolioID string) ([]Section, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, portfolio_id, type, title, sort_order, is_visible, created_at, updated_at
		FROM sections WHERE portfolio_id = $1 ORDER BY sort_order ASC
	`, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []Section
	for rows.Next() {
		var s Section
		if err := rows.Scan(&s.ID, &s.PortfolioID, &s.Type, &s.Title, &s.SortOrder, &s.IsVisible, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sections = append(sections, s)
	}
	return sections, rows.Err()
}

func UpdateSectionOrder(ctx context.Context, pool *pgxpool.Pool, portfolioID string, orderedIDs []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, id := range orderedIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE sections SET sort_order = $1, updated_at = NOW() WHERE id = $2 AND portfolio_id = $3
		`, i, id, portfolioID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ToggleSectionVisibility(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string) error {
	_, err := pool.Exec(ctx, `
		UPDATE sections SET is_visible = NOT is_visible, updated_at = NOW()
		WHERE id = $1 AND portfolio_id = $2
	`, id, portfolioID)
	return err
}

func UpdateSectionTitle(ctx context.Context, pool *pgxpool.Pool, id, portfolioID, title string) error {
	_, err := pool.Exec(ctx, `
		UPDATE sections SET title = $1, updated_at = NOW() WHERE id = $2 AND portfolio_id = $3
	`, title, id, portfolioID)
	return err
}

func DeleteSection(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string) error {
	_, err := pool.Exec(ctx, `DELETE FROM sections WHERE id = $1 AND portfolio_id = $2`, id, portfolioID)
	return err
}

// FindSectionByPortfolioAndType returns the first section of the given type
// on a portfolio, used to merge re-uploaded CV data into an existing section
// instead of creating a duplicate.
func FindSectionByPortfolioAndType(ctx context.Context, pool *pgxpool.Pool, portfolioID, sectionType string) (*Section, error) {
	s := &Section{}
	err := pool.QueryRow(ctx, `
		SELECT id, portfolio_id, type, title, sort_order, is_visible, created_at, updated_at
		FROM sections WHERE portfolio_id = $1 AND type = $2
		ORDER BY sort_order ASC LIMIT 1
	`, portfolioID, sectionType).Scan(&s.ID, &s.PortfolioID, &s.Type, &s.Title, &s.SortOrder, &s.IsVisible, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// FindCustomSectionByTitle returns an existing custom section with a matching
// title on a portfolio, if any.
func FindCustomSectionByTitle(ctx context.Context, pool *pgxpool.Pool, portfolioID, title string) (*Section, error) {
	s := &Section{}
	err := pool.QueryRow(ctx, `
		SELECT id, portfolio_id, type, title, sort_order, is_visible, created_at, updated_at
		FROM sections WHERE portfolio_id = $1 AND type = 'custom' AND title = $2
		LIMIT 1
	`, portfolioID, title).Scan(&s.ID, &s.PortfolioID, &s.Type, &s.Title, &s.SortOrder, &s.IsVisible, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// NextSectionSortOrder returns the sort_order to use for a newly appended section.
func NextSectionSortOrder(ctx context.Context, pool *pgxpool.Pool, portfolioID string) (int, error) {
	var max *int
	err := pool.QueryRow(ctx, `SELECT MAX(sort_order) FROM sections WHERE portfolio_id = $1`, portfolioID).Scan(&max)
	if err != nil {
		return 0, err
	}
	if max == nil {
		return 0, nil
	}
	return *max + 1, nil
}

// CountItemsInSection returns how many items a section currently has, used
// to append new items after existing ones during a merge.
func CountItemsInSection(ctx context.Context, pool *pgxpool.Pool, sectionID string) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM section_items WHERE section_id = $1`, sectionID).Scan(&count)
	return count, err
}
