package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Image struct {
	ID          string
	PortfolioID string
	Filename    string
	Kind        string
	Width       int
	Height      int
	CreatedAt   time.Time
}

func CreateImage(ctx context.Context, pool *pgxpool.Pool, id, portfolioID, filename, kind string, width, height int) (*Image, error) {
	img := &Image{}
	err := pool.QueryRow(ctx, `
		INSERT INTO images (id, portfolio_id, filename, kind, width, height)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, portfolio_id, filename, kind, width, height, created_at
	`, id, portfolioID, filename, kind, width, height).Scan(
		&img.ID, &img.PortfolioID, &img.Filename, &img.Kind, &img.Width, &img.Height, &img.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func FindImageByID(ctx context.Context, pool *pgxpool.Pool, id string) (*Image, error) {
	img := &Image{}
	err := pool.QueryRow(ctx, `
		SELECT id, portfolio_id, filename, kind, width, height, created_at FROM images WHERE id = $1
	`, id).Scan(&img.ID, &img.PortfolioID, &img.Filename, &img.Kind, &img.Width, &img.Height, &img.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return img, nil
}

func DeleteImage(ctx context.Context, pool *pgxpool.Pool, id, portfolioID string) error {
	_, err := pool.Exec(ctx, `DELETE FROM images WHERE id = $1 AND portfolio_id = $2`, id, portfolioID)
	return err
}
