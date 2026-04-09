package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

var (
	handleNonAlnum = regexp.MustCompile(`[^a-z0-9_.-]`)
	handleMulti    = regexp.MustCompile(`_+`)
)

type UserRepository struct{ db *DB }

func NewUserRepository(db *DB) *UserRepository { return &UserRepository{db: db} }

// Create inserts a new user, auto-deriving a unique @mention handle from display_name.
func (r *UserRepository) Create(ctx context.Context, email, displayName, passwordHash string) (*models.User, error) {
	handle := deriveHandle(displayName)

	for attempt := 0; attempt < 5; attempt++ {
		candidate := handle
		if attempt > 0 {
			candidate = fmt.Sprintf("%s_%04d", handle, rand.Intn(9999)+1)
		}

		u := &models.User{}
		err := r.db.QueryRow(ctx, `
			INSERT INTO users (email, display_name, password_hash, handle)
			VALUES ($1, $2, $3, $4)
			RETURNING id, email, display_name, handle, avatar_url, created_at, updated_at`,
			email, displayName, passwordHash, candidate,
		).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Handle, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)

		if err == nil {
			return u, nil
		}

		// if it's a unique violation on the handle, try a different suffix
		if strings.Contains(err.Error(), "idx_users_handle") || strings.Contains(err.Error(), "users_handle") {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("could not generate unique handle for %q", displayName)
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, display_name, handle, avatar_url, password_hash, created_at, updated_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Handle, &u.AvatarURL, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, display_name, handle, avatar_url, password_hash, created_at, updated_at
		FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Handle, &u.AvatarURL, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// GetByHandle looks up a user by their @mention handle.
func (r *UserRepository) GetByHandle(ctx context.Context, handle string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, email, display_name, handle, avatar_url, password_hash, created_at, updated_at
		FROM users WHERE handle = $1`, handle,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Handle, &u.AvatarURL, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *UserRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.UserResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, email, display_name, handle, avatar_url
		FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.UserResponse
	for rows.Next() {
		u := &models.UserResponse{}
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Handle, &u.AvatarURL); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// deriveHandle converts a display name into a lowercase handle-safe string.
func deriveHandle(displayName string) string {
	h := strings.ToLower(strings.TrimSpace(displayName))
	h = strings.ReplaceAll(h, " ", "_")
	h = handleNonAlnum.ReplaceAllString(h, "")
	h = handleMulti.ReplaceAllString(h, "_")
	h = strings.Trim(h, "_.")
	if len(h) > 46 { // leave room for "_9999" suffix
		h = h[:46]
	}
	if h == "" {
		h = "user"
	}
	return h
}
