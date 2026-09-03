package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userStore struct {
	pool *pgxpool.Pool
}

func (u *userStore) User(ctx context.Context, userID string) (*store.User, error) {
	def := store.DefaultUser(userID)

	_, err := u.pool.Exec(ctx, `INSERT INTO users (id, dm, crosspost, ignore, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING`,
		def.ID, def.DM, def.Crosspost, def.Ignore, def.CreatedAt, def.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	user, err := u.loadUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u *userStore) CreateUser(ctx context.Context, id string) (*store.User, error) {
	user := store.DefaultUser(id)

	_, err := u.pool.Exec(ctx, `INSERT INTO users (id, dm, crosspost, ignore, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		user.ID, user.DM, user.Crosspost, user.Ignore, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	user.Groups = make([]*store.Group, 0)

	return user, nil
}

func (u *userStore) UpdateUser(ctx context.Context, user *store.User) (*store.User, error) {
	user.UpdatedAt = time.Now().UTC()

	_, err := u.pool.Exec(ctx, `UPDATE users SET dm=$2, crosspost=$3, ignore=$4, updated_at=$5 WHERE id=$1`,
		user.ID, user.DM, user.Crosspost, user.Ignore, user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (u *userStore) CreateCrosspostGroup(ctx context.Context, userID string, group *store.Group) (*store.User, error) {
	children := group.Children
	if children == nil {
		children = make([]string, 0)
	}

	_, err := u.pool.Exec(ctx, `INSERT INTO user_groups (user_id, name, parent, is_pair, children)
		VALUES ($1,$2,$3,FALSE,$4)`,
		userID, group.Name, group.Parent, children,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return u.loadUser(ctx, userID)
		}

		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) CreateCrosspostPair(ctx context.Context, userID string, pair *store.Group) (*store.User, error) {
	children := pair.Children
	if children == nil {
		children = make([]string, 0)
	}

	_, err := u.pool.Exec(ctx, `INSERT INTO user_groups (user_id, name, parent, is_pair, children)
		VALUES ($1,$2,'',TRUE,$3)`,
		userID, pair.Name, children,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return u.loadUser(ctx, userID)
		}

		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) DeleteCrosspostGroup(ctx context.Context, userID, group string) (*store.User, error) {
	_, err := u.pool.Exec(ctx, `DELETE FROM user_groups WHERE user_id=$1 AND name=$2`, userID, group)
	if err != nil {
		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) RenameCrosspostGroup(ctx context.Context, userID, group, rename string) (*store.User, error) {
	_, err := u.pool.Exec(ctx, `UPDATE user_groups SET name=$3 WHERE user_id=$1 AND name=$2`, userID, group, rename)
	if err != nil {
		if isUniqueViolation(err) {
			return u.loadUser(ctx, userID)
		}

		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) AddCrosspostChannel(ctx context.Context, userID, group, child string) (*store.User, error) {
	_, err := u.pool.Exec(ctx, `UPDATE user_groups SET children = (
			SELECT array_agg(DISTINCT ch) FROM unnest(children || $3) AS ch WHERE ch IS NOT NULL
		) WHERE user_id=$1 AND name=$2`, userID, group, []string{child},
	)
	if err != nil {
		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) DeleteCrosspostChannel(ctx context.Context, userID, group, child string) (*store.User, error) {
	_, err := u.pool.Exec(ctx, `UPDATE user_groups SET children = COALESCE((
			SELECT array_agg(ch) FROM (
				SELECT unnest(children) AS ch EXCEPT SELECT unnest($3::text[])
			) s WHERE ch IS NOT NULL
		), '{}') WHERE user_id=$1 AND name=$2`, userID, group, []string{child},
	)
	if err != nil {
		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) EditCrosspostParent(ctx context.Context, userID, group, parent string) (*store.User, error) {
	_, err := u.pool.Exec(ctx, `UPDATE user_groups SET parent=$3 WHERE user_id=$1 AND name=$2`, userID, group, parent)
	if err != nil {
		return nil, err
	}

	return u.loadUser(ctx, userID)
}

func (u *userStore) loadUser(ctx context.Context, userID string) (*store.User, error) {
	row := u.pool.QueryRow(ctx, `SELECT id, dm, crosspost, ignore, created_at, updated_at FROM users WHERE id=$1`, userID)

	user := &store.User{}
	if err := row.Scan(&user.ID, &user.DM, &user.Crosspost, &user.Ignore, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}

		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	rows, err := u.pool.Query(ctx, `SELECT name, parent, is_pair, children FROM user_groups WHERE user_id=$1 ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	user.Groups = make([]*store.Group, 0)
	for rows.Next() {
		group := &store.Group{}
		if err := rows.Scan(&group.Name, &group.Parent, &group.IsPair, &group.Children); err != nil {
			return nil, err
		}

		if group.Children == nil {
			group.Children = make([]string, 0)
		}

		user.Groups = append(user.Groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return user, nil
}
