package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SectionItem struct {
	ID          string
	SectionID   string
	SortOrder   int
	Title       *string
	Subtitle    *string
	Location    *string
	StartDate   *string
	EndDate     *string
	Description *string
	URL         *string
	Meta        json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func CreateItem(ctx context.Context, pool *pgxpool.Pool, sectionID string, item SectionItem) (*SectionItem, error) {
	result := &SectionItem{}
	err := pool.QueryRow(ctx, `
		INSERT INTO section_items
			(section_id, sort_order, title, subtitle, location, start_date, end_date, description, url, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, section_id, sort_order, title, subtitle, location, start_date, end_date, description, url, meta, created_at, updated_at
	`, sectionID, item.SortOrder, item.Title, item.Subtitle, item.Location,
		item.StartDate, item.EndDate, item.Description, item.URL, item.Meta,
	).Scan(
		&result.ID, &result.SectionID, &result.SortOrder, &result.Title, &result.Subtitle,
		&result.Location, &result.StartDate, &result.EndDate, &result.Description,
		&result.URL, &result.Meta, &result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ListItemsBySection(ctx context.Context, pool *pgxpool.Pool, sectionID string) ([]SectionItem, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, section_id, sort_order, title, subtitle, location, start_date, end_date, description, url, meta, created_at, updated_at
		FROM section_items WHERE section_id = $1 ORDER BY sort_order ASC
	`, sectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SectionItem
	for rows.Next() {
		var it SectionItem
		if err := rows.Scan(
			&it.ID, &it.SectionID, &it.SortOrder, &it.Title, &it.Subtitle,
			&it.Location, &it.StartDate, &it.EndDate, &it.Description,
			&it.URL, &it.Meta, &it.CreatedAt, &it.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func UpdateItem(ctx context.Context, pool *pgxpool.Pool, item SectionItem) error {
	_, err := pool.Exec(ctx, `
		UPDATE section_items SET
			title = $1, subtitle = $2, location = $3, start_date = $4,
			end_date = $5, description = $6, url = $7, meta = $8, updated_at = NOW()
		WHERE id = $9 AND section_id = $10
	`, item.Title, item.Subtitle, item.Location, item.StartDate,
		item.EndDate, item.Description, item.URL, item.Meta, item.ID, item.SectionID)
	return err
}

func DeleteItem(ctx context.Context, pool *pgxpool.Pool, id, sectionID string) error {
	_, err := pool.Exec(ctx, `DELETE FROM section_items WHERE id = $1 AND section_id = $2`, id, sectionID)
	return err
}

func UpdateItemOrder(ctx context.Context, pool *pgxpool.Pool, sectionID string, orderedIDs []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, id := range orderedIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE section_items SET sort_order = $1, updated_at = NOW() WHERE id = $2 AND section_id = $3
		`, i, id, sectionID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
