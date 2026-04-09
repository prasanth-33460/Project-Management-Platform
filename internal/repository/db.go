package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/redis/go-redis/v9"
)

// DB is the shared alias for the pgx connection pool used across all repositories.
type DB = pgxpool.Pool

// Database is the same type exposed via the IssueStore.Pool() interface method.
type Database = pgxpool.Pool

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

func NewRedis(redisURL string) *redis.Client {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("invalid redis URL, falling back to localhost:6379", "error", err)
		opts = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opts)
}

// RunMigrations applies each SQL file in order, skipping files already recorded
// in schema_migrations. Safe to call on every startup.
//
// If postgres initdb.d seeded the schema before the app ran for the first time,
// the migration SQL will hit "already exists" errors. We treat that as a bootstrap
// signal — mark the file applied and move on rather than aborting.
func RunMigrations(ctx context.Context, pool *Database, files ...string) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, file := range files {
		var already bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)`, file,
		).Scan(&already); err != nil {
			return fmt.Errorf("check migration %q: %w", file, err)
		}
		if already {
			slog.Info("migration already applied, skipping", "file", file)
			continue
		}

		sql, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration file %q: %w", file, err)
		}

		if _, execErr := pool.Exec(ctx, string(sql)); execErr != nil {
			if strings.Contains(execErr.Error(), "already exists") {
				slog.Warn("migration objects already exist, marking applied", "file", file)
			} else {
				return fmt.Errorf("execute migration %q: %w", file, execErr)
			}
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, file,
		); err != nil {
			return fmt.Errorf("record migration %q: %w", file, err)
		}
		slog.Info("migration applied", "file", file)
	}
	return nil
}

type dbTransactor struct{ pool *pgxpool.Pool }

func NewTransactor(pool *Database) Transactor {
	return &dbTransactor{pool: pool}
}

func (t *dbTransactor) WithTx(ctx context.Context, fn func(ctx context.Context, tx TxStore) error) error {
	pgxTx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// rollback is a no-op after a successful commit
	defer func() {
		if rbErr := pgxTx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			slog.Error("transaction rollback failed", "error", rbErr)
		}
	}()

	if err := fn(ctx, &txRepository{tx: pgxTx}); err != nil {
		return err
	}
	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// txRepository implements TxStore against an in-flight pgx.Tx.
type txRepository struct{ tx pgx.Tx }

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

func (r *txRepository) UpdateIssueSprint(ctx context.Context, issueID uuid.UUID, sprintID *uuid.UUID) error {
	_, err := r.tx.Exec(ctx,
		`UPDATE issues SET sprint_id = $1, updated_at = NOW() WHERE id = $2`,
		sprintID, issueID)
	if err != nil {
		return fmt.Errorf("update issue sprint: %w", err)
	}
	return nil
}

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

// Repositories bundles all store implementations for injection into services.
type Repositories struct {
	User         UserStore
	Project      ProjectStore
	Issue        IssueStore
	Sprint       SprintStore
	Workflow     WorkflowStore
	Comment      CommentStore
	Notification NotificationStore
	CustomField  CustomFieldStore
	Tx           Transactor
}

func NewRepositories(db *Database) *Repositories {
	return &Repositories{
		User:         NewUserRepository(db),
		Project:      NewProjectRepository(db),
		Issue:        NewIssueRepository(db),
		Sprint:       NewSprintRepository(db),
		Workflow:     NewWorkflowRepository(db),
		Comment:      NewCommentRepository(db),
		Notification: NewNotificationRepository(db),
		CustomField:  NewCustomFieldRepository(db),
		Tx:           NewTransactor(db),
	}
}
