package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/redis/go-redis/v9"
)

// Database / DB are both aliases for the underlying pgx pool.
// DB is used internally by all repository structs; Database is used in interfaces.
type Database = pgxpool.Pool
type DB = pgxpool.Pool

// NewDB creates and validates a PostgreSQL connection pool.
// It configures sensible production defaults for connection counts and lifetimes.
func NewDB(ctx context.Context, connString string) (*Database, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	cfg.MaxConns = 30
	cfg.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// NewRedis creates a Redis client from a URL string.
func NewRedis(redisURL string) *redis.Client {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("invalid redis URL, falling back to localhost:6379", "error", err)
		opts = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opts)
}

// RunMigrations executes the init migration SQL only when the schema has not yet
// been applied (checked by the presence of the 'users' table). This is a simple
// idempotent approach appropriate for single-binary deployments; use a proper
// migration tool (golang-migrate, goose) for multi-version schemas.
func RunMigrations(ctx context.Context, pool *Database, migrationFile string) error {
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'users'
		)`).Scan(&exists); err != nil {
		return fmt.Errorf("check migration state: %w", err)
	}
	if exists {
		slog.Info("database already migrated, skipping")
		return nil
	}

	sql, err := os.ReadFile(migrationFile)
	if err != nil {
		return fmt.Errorf("read migration file %q: %w", migrationFile, err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}
	slog.Info("database migration applied", "file", migrationFile)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Transaction support
// ─────────────────────────────────────────────────────────────────────────────

// dbTransactor implements the Transactor interface using pgxpool.
type dbTransactor struct{ pool *pgxpool.Pool }

// NewTransactor wraps a pool into a Transactor.
func NewTransactor(pool *Database) Transactor {
	return &dbTransactor{pool: pool}
}

// WithTx begins a transaction, calls fn, and commits.
// Any error from fn causes an automatic rollback.
func (t *dbTransactor) WithTx(ctx context.Context, fn func(ctx context.Context, tx TxStore) error) error {
	pgxTx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Deferred rollback is a no-op if Commit has already been called.
	defer func() {
		if rbErr := pgxTx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			slog.Error("transaction rollback failed", "error", rbErr)
		}
	}()

	txStore := &txRepository{tx: pgxTx}
	if err := fn(ctx, txStore); err != nil {
		return err
	}
	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// txRepository — implements TxStore using an in-flight pgx.Tx
// ─────────────────────────────────────────────────────────────────────────────

type txRepository struct{ tx pgx.Tx }

// UpdateIssueStatus applies an optimistic-locked status change in the current Tx.
func (r *txRepository) UpdateIssueStatus(ctx context.Context, issueID, statusID uuid.UUID, version int) (int64, error) {
	tag, err := r.tx.Exec(ctx, `
		UPDATE issues
		SET status_id  = $1,
		    version    = version + 1,
		    updated_at = NOW()
		WHERE id = $2 AND version = $3`,
		statusID, issueID, version)
	if err != nil {
		return 0, fmt.Errorf("update issue status: %w", err)
	}
	return tag.RowsAffected(), nil
}

// UpdateIssueSprint moves an issue to the given sprint (nil means backlog) in the current Tx.
func (r *txRepository) UpdateIssueSprint(ctx context.Context, issueID uuid.UUID, sprintID *uuid.UUID) error {
	_, err := r.tx.Exec(ctx,
		`UPDATE issues SET sprint_id = $1, updated_at = NOW() WHERE id = $2`,
		sprintID, issueID)
	if err != nil {
		return fmt.Errorf("update issue sprint: %w", err)
	}
	return nil
}

// LogActivity inserts an immutable audit entry in the current Tx.
func (r *txRepository) LogActivity(ctx context.Context, entry *models.ActivityLog) error {
	_, err := r.tx.Exec(ctx, `
		INSERT INTO activity_log
		    (issue_id, project_id, actor_id, event_type, field_name, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.IssueID, entry.ProjectID, entry.ActorID, entry.EventType,
		entry.FieldName, entry.OldValue, entry.NewValue)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Repositories container
// ─────────────────────────────────────────────────────────────────────────────

// Repositories bundles all store implementations for injection into services.
type Repositories struct {
	User         UserStore
	Project      ProjectStore
	Issue        IssueStore
	Sprint       SprintStore
	Workflow     WorkflowStore
	Comment      CommentStore
	Notification NotificationStore
	Tx           Transactor
}

// NewRepositories wires concrete implementations against the given pool.
func NewRepositories(db *Database) *Repositories {
	return &Repositories{
		User:         NewUserRepository(db),
		Project:      NewProjectRepository(db),
		Issue:        NewIssueRepository(db),
		Sprint:       NewSprintRepository(db),
		Workflow:     NewWorkflowRepository(db),
		Comment:      NewCommentRepository(db),
		Notification: NewNotificationRepository(db),
		Tx:           NewTransactor(db),
	}
}
