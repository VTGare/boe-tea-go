package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type artworkStore struct {
	pool *pgxpool.Pool
}

func (a *artworkStore) Artwork(ctx context.Context, id int, url string) (*store.Artwork, error) {
	if id == 0 && url == "" {
		return nil, store.ErrArtworkNotFound
	}

	query := `SELECT id, title, author, url, images, favourites, created_at, updated_at FROM artworks WHERE id = $1 AND url = $2`
	args := []any{id, url}

	switch {
	case id != 0 && url != "":
		// keep both predicates
	case id != 0:
		query = `SELECT id, title, author, url, images, favourites, created_at, updated_at FROM artworks WHERE id = $1`
		args = []any{id}
	default:
		query = `SELECT id, title, author, url, images, favourites, created_at, updated_at FROM artworks WHERE url = $1`
		args = []any{url}
	}

	row := a.pool.QueryRow(ctx, query, args...)

	artwork := &store.Artwork{}
	if err := scanArtwork(row, artwork); err != nil {
		return nil, err
	}

	return artwork, nil
}

func (a *artworkStore) CreateArtwork(ctx context.Context, artwork *store.Artwork) (*store.Artwork, error) {
	now := time.Now().UTC()

	images := artwork.Images
	if images == nil {
		images = make([]string, 0)
	}

	row := a.pool.QueryRow(ctx, `INSERT INTO artworks (id, title, author, url, images, favourites, created_at, updated_at)
		VALUES (nextval('artwork_id_seq'), $1, $2, $3, $4, 0, $5, $6)
		RETURNING id, title, author, url, images, favourites, created_at, updated_at`,
		artwork.Title, artwork.Author, artwork.URL, images, now, now,
	)

	created := &store.Artwork{}
	if err := scanArtwork(row, created); err != nil {
		return nil, err
	}

	return created, nil
}

func (a *artworkStore) SearchArtworks(ctx context.Context, filter store.ArtworkFilter, opts ...store.ArtworkSearchOptions) ([]*store.Artwork, error) {
	opt := store.DefaultSearchOptions()
	if len(opts) != 0 {
		opt = opts[0]
	}

	where, args := artworkWhere(filter)

	order := "DESC"
	if opt.Order == store.Ascending {
		order = "ASC"
	}

	sortCol := "created_at"
	if opt.Sort == store.ByPopularity {
		sortCol = "favourites"
	}

	query := fmt.Sprintf(`SELECT id, title, author, url, images, favourites, created_at, updated_at FROM artworks %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, sortCol, order, len(args)+1, len(args)+2,
	)
	args = append(args, opt.Limit, opt.Skip)

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artworks := make([]*store.Artwork, 0)
	for rows.Next() {
		artwork := &store.Artwork{}
		if err := scanArtwork(rows, artwork); err != nil {
			return nil, err
		}

		artworks = append(artworks, artwork)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return artworks, nil
}

func artworkWhere(f store.ArtworkFilter) (string, []any) {
	switch {
	case len(f.IDs) != 0:
		return `WHERE id = ANY($1)`, []any{f.IDs}
	case f.URL != "":
		return `WHERE url = $1`, []any{f.URL}
	case f.Query != "":
		like := "%" + escapeILIKE(f.Query) + "%"

		return `WHERE author ILIKE $1 ESCAPE '\' OR title ILIKE $1 ESCAPE '\'`, []any{like}
	default:
		clauses := make([]string, 0)
		args := make([]any, 0)

		if f.Author != "" {
			args = append(args, "%"+escapeILIKE(f.Author)+"%")
			clauses = append(clauses, fmt.Sprintf(`author ILIKE $%d ESCAPE '\'`, len(args)))
		}

		if f.Title != "" {
			args = append(args, "%"+escapeILIKE(f.Title)+"%")
			clauses = append(clauses, fmt.Sprintf(`title ILIKE $%d ESCAPE '\'`, len(args)))
		}

		if f.Time != 0 {
			args = append(args, time.Now().Add(-f.Time))
			clauses = append(clauses, fmt.Sprintf(`created_at >= $%d`, len(args)))
		}

		if len(clauses) == 0 {
			return "", nil
		}

		return "WHERE " + strings.Join(clauses, " AND "), args
	}
}

type artworkRow interface {
	Scan(dest ...any) error
}

func scanArtwork(row artworkRow, a *store.Artwork) error {
	if err := row.Scan(&a.ID, &a.Title, &a.Author, &a.URL, &a.Images, &a.Favorites, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return store.ErrArtworkNotFound
		}

		return fmt.Errorf("failed to scan artwork: %w", err)
	}

	if a.Images == nil {
		a.Images = make([]string, 0)
	}

	return nil
}
