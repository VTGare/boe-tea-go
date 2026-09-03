package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type bookmarkStore struct {
	pool *pgxpool.Pool
}

func (b *bookmarkStore) ListBookmarks(ctx context.Context, userID string, filter store.BookmarkFilter, order store.Order, opts ...store.BookmarkListOptions) ([]*store.Bookmark, error) {
	query := `SELECT user_id, artwork_id, nsfw, created_at FROM bookmarks WHERE user_id = $1`
	args := []any{userID}

	if filter != store.BookmarkFilterAll {
		args = append(args, filter == store.BookmarkFilterUnsafe)
		query += fmt.Sprintf(` AND nsfw = $%d`, len(args))
	}

	direction := "DESC"
	if order == store.Ascending {
		direction = "ASC"
	}

	query += ` ORDER BY created_at ` + direction

	if len(opts) != 0 {
		if opts[0].Limit > 0 {
			args = append(args, opts[0].Limit)
			query += fmt.Sprintf(` LIMIT $%d`, len(args))
		}

		if opts[0].Skip > 0 {
			args = append(args, opts[0].Skip)
			query += fmt.Sprintf(` OFFSET $%d`, len(args))
		}
	}

	rows, err := b.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to find bookmarks: %w", err)
	}
	defer rows.Close()

	bookmarks := make([]*store.Bookmark, 0)
	for rows.Next() {
		bookmark := &store.Bookmark{}
		if err := rows.Scan(&bookmark.UserID, &bookmark.ArtworkID, &bookmark.NSFW, &bookmark.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan bookmark: %w", err)
		}

		bookmarks = append(bookmarks, bookmark)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list bookmarks: %w", err)
	}

	return bookmarks, nil
}

func (b *bookmarkStore) CountBookmarks(ctx context.Context, userID string) (int64, error) {
	var count int64
	if err := b.pool.QueryRow(ctx, `SELECT count(*) FROM bookmarks WHERE user_id=$1`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count bookmarks: %w", err)
	}

	return count, nil
}

func (b *bookmarkStore) AddBookmark(ctx context.Context, bookmark *store.Bookmark) (bool, error) {
	if bookmark.CreatedAt.IsZero() {
		bookmark.CreatedAt = time.Now().UTC()
	}

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `INSERT INTO bookmarks (user_id, artwork_id, nsfw, created_at)
		VALUES ($1,$2,$3,$4) ON CONFLICT (user_id, artwork_id) DO NOTHING`,
		bookmark.UserID, bookmark.ArtworkID, bookmark.NSFW, bookmark.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("failed to insert bookmark: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err := tx.Exec(ctx, `UPDATE artworks SET favourites = favourites + 1 WHERE id = $1`, bookmark.ArtworkID); err != nil {
		return false, fmt.Errorf("failed to increment artwork favorite count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	return true, nil
}

func (b *bookmarkStore) DeleteBookmark(ctx context.Context, bookmark *store.Bookmark) (bool, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM bookmarks WHERE user_id=$1 AND artwork_id=$2`, bookmark.UserID, bookmark.ArtworkID)
	if err != nil {
		return false, fmt.Errorf("failed to delete bookmark: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err := tx.Exec(ctx, `UPDATE artworks SET favourites = GREATEST(favourites - 1, 0) WHERE id = $1`, bookmark.ArtworkID); err != nil {
		return false, fmt.Errorf("failed to decrement artwork favorite count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	return true, nil
}
