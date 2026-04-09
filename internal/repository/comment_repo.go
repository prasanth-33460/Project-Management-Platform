package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type CommentRepository struct{ db *DB }

func NewCommentRepository(db *DB) *CommentRepository { return &CommentRepository{db: db} }

func (r *CommentRepository) Create(ctx context.Context, issueID, authorID uuid.UUID, body string, parentID *uuid.UUID) (*models.Comment, error) {
	c := &models.Comment{Author: &models.UserResponse{}}
	err := r.db.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO comments (issue_id, author_id, body, parent_id)
			VALUES ($1, $2, $3, $4)
			RETURNING id, issue_id, author_id, body, parent_id, created_at, updated_at
		)
		SELECT ins.id, ins.issue_id, ins.author_id, ins.body, ins.parent_id, ins.created_at, ins.updated_at,
		       u.id, u.email, u.display_name, u.avatar_url
		FROM ins
		JOIN users u ON u.id = ins.author_id`,
		issueID, authorID, body, parentID,
	).Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.ParentID, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.ID, &c.Author.Email, &c.Author.DisplayName, &c.Author.AvatarURL)
	return c, err
}

func (r *CommentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Comment, error) {
	c := &models.Comment{Author: &models.UserResponse{}}
	err := r.db.QueryRow(ctx, `
		SELECT c.id, c.issue_id, c.author_id, c.body, c.parent_id, c.created_at, c.updated_at,
		       u.id, u.email, u.display_name, u.avatar_url
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.id = $1`, id,
	).Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.ParentID, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.ID, &c.Author.Email, &c.Author.DisplayName, &c.Author.AvatarURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (r *CommentRepository) ListByIssue(ctx context.Context, issueID uuid.UUID) ([]*models.Comment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.issue_id, c.author_id, c.body, c.parent_id, c.created_at, c.updated_at,
		       u.id, u.email, u.display_name, u.avatar_url
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.issue_id = $1
		ORDER BY c.created_at ASC`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commentMap := map[uuid.UUID]*models.Comment{}
	var roots []*models.Comment

	for rows.Next() {
		c := &models.Comment{Author: &models.UserResponse{}}
		if err := rows.Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.ParentID, &c.CreatedAt, &c.UpdatedAt,
			&c.Author.ID, &c.Author.Email, &c.Author.DisplayName, &c.Author.AvatarURL); err != nil {
			return nil, err
		}
		commentMap[c.ID] = c
		if c.ParentID == nil {
			roots = append(roots, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach replies to parents
	for _, c := range commentMap {
		if c.ParentID != nil {
			if parent, ok := commentMap[*c.ParentID]; ok {
				parent.Replies = append(parent.Replies, c)
			}
		}
	}
	return roots, nil
}

func (r *CommentRepository) Update(ctx context.Context, id uuid.UUID, body string) (*models.Comment, error) {
	c := &models.Comment{Author: &models.UserResponse{}}
	err := r.db.QueryRow(ctx, `
		WITH upd AS (
			UPDATE comments SET body = $2, updated_at = NOW()
			WHERE id = $1
			RETURNING id, issue_id, author_id, body, parent_id, created_at, updated_at
		)
		SELECT upd.id, upd.issue_id, upd.author_id, upd.body, upd.parent_id, upd.created_at, upd.updated_at,
		       u.id, u.email, u.display_name, u.avatar_url
		FROM upd JOIN users u ON u.id = upd.author_id`, id, body,
	).Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.Body, &c.ParentID, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.ID, &c.Author.Email, &c.Author.DisplayName, &c.Author.AvatarURL)
	return c, err
}

func (r *CommentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	return err
}
