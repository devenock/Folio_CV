package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Block struct {
	ID          string
	PortfolioID string
	Type        string
	GridX       int
	GridY       int
	GridW       int
	GridH       int
	ZIndex      int
	IsVisible   bool
	Content     json.RawMessage
	Style       json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const blockColumns = `
	id, portfolio_id, type, grid_x, grid_y, grid_w, grid_h, z_index,
	is_visible, content, style, created_at, updated_at
`

func scanBlock(row pgx.Row) (*Block, error) {
	b := &Block{}
	err := row.Scan(
		&b.ID, &b.PortfolioID, &b.Type, &b.GridX, &b.GridY, &b.GridW, &b.GridH,
		&b.ZIndex, &b.IsVisible, &b.Content, &b.Style, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return b, nil
}

func CreateBlock(ctx context.Context, pool *pgxpool.Pool, portfolioID, blockType string, gridX, gridY, gridW, gridH int, content json.RawMessage) (*Block, error) {
	if content == nil {
		content = json.RawMessage(`{}`)
	}
	row := pool.QueryRow(ctx, `
		INSERT INTO blocks (portfolio_id, type, grid_x, grid_y, grid_w, grid_h, content)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+blockColumns,
		portfolioID, blockType, gridX, gridY, gridW, gridH, content,
	)
	return scanBlock(row)
}

func FindBlockByID(ctx context.Context, pool *pgxpool.Pool, id string) (*Block, error) {
	row := pool.QueryRow(ctx, `SELECT `+blockColumns+` FROM blocks WHERE id = $1`, id)
	return scanBlock(row)
}

func ListBlocksByPortfolio(ctx context.Context, pool *pgxpool.Pool, portfolioID string) ([]Block, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+blockColumns+` FROM blocks WHERE portfolio_id = $1 ORDER BY grid_y ASC, grid_x ASC
	`, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []Block
	for rows.Next() {
		var b Block
		if err := rows.Scan(
			&b.ID, &b.PortfolioID, &b.Type, &b.GridX, &b.GridY, &b.GridW, &b.GridH,
			&b.ZIndex, &b.IsVisible, &b.Content, &b.Style, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// UpdateBlockPosition persists only the grid geometry — called frequently
// (on every drag/resize end), kept separate from content updates so a fast
// position change can't race with a slower content-form submit.
func UpdateBlockPosition(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string, gridX, gridY, gridW, gridH, zIndex int) error {
	_, err := pool.Exec(ctx, `
		UPDATE blocks SET grid_x = $1, grid_y = $2, grid_w = $3, grid_h = $4, z_index = $5, updated_at = NOW()
		WHERE id = $6 AND portfolio_id = $7
	`, gridX, gridY, gridW, gridH, zIndex, id, portfolioID)
	return err
}

func UpdateBlockContent(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string, content json.RawMessage) error {
	_, err := pool.Exec(ctx, `
		UPDATE blocks SET content = $1, updated_at = NOW() WHERE id = $2 AND portfolio_id = $3
	`, content, id, portfolioID)
	return err
}

func ToggleBlockVisibility(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string) error {
	_, err := pool.Exec(ctx, `
		UPDATE blocks SET is_visible = NOT is_visible, updated_at = NOW()
		WHERE id = $1 AND portfolio_id = $2
	`, id, portfolioID)
	return err
}

func DeleteBlock(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string) error {
	_, err := pool.Exec(ctx, `DELETE FROM blocks WHERE id = $1 AND portfolio_id = $2`, id, portfolioID)
	return err
}
