package repository

import (
	"CommentTree/internal/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/retry"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Repository struct {
	db  *dbpg.DB
	log *zap.Logger
}

const (
	createQuery                = `INSERT INTO comments (parent_id,path_id,path,comment,created_at) VALUES ($1,$2,$3,$4,$5)`
	getPathQuery               = `SELECT path FROM comments WHERE id = $1`
	getCommentByIDQuery        = `SELECT id,parent_id,path_id,path,comment,created_at FROM comments WHERE id = $1`
	getChildrenByPathQuery     = `SELECT id,parent_id,path_id,path,comment,created_at FROM comments WHERE path LIKE $1 ORDER BY path DESC`
	deleteCommentQuery         = `DELETE FROM comments WHERE path LIKE $1`
	countTopLevelCommentsQuery = `SELECT COUNT(*) FROM comments WHERE parent_id IS NULL`
	searchCommentsQuery        = `SELECT id, parent_id, path_id, path, comment, created_at FROM comments WHERE comment ILIKE $1 ORDER BY created_at DESC LIMIT 50`
)

var (
	retryStrategy = retry.Strategy{
		Attempts: 5,
		Delay:    time.Millisecond,
		Backoff:  2,
	}
)

func NewRepository(masterDSN string, slaveDSNs []string, log *zap.Logger) (*Repository, error) {
	opts := dbpg.Options{
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}
	db, err := dbpg.New(masterDSN, slaveDSNs, &opts)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Info("Starting database migrations")

	if err := runMigrations(masterDSN); err != nil {
		log.Error("Failed to run migrations", zap.Error(err))
		return nil, fmt.Errorf("failed to run migration: %w", err)
	}
	log.Info("Successfully migrated database")

	return &Repository{db: db, log: log.Named("repository")}, nil
}

func (r *Repository) Create(ctx context.Context, c models.Comment) (int64, error) {
	var parentPath string

	if c.ParentID != nil {
		row, err := r.db.QueryRowWithRetry(ctx, retryStrategy, getPathQuery, *c.ParentID)
		if err != nil {
			r.log.Error("QueryRow for parent comment failed", zap.Error(err), zap.Int64("parent_id", *c.ParentID))
			return -1, fmt.Errorf("query for parent comment failed: %w", err)
		}

		if err := row.Scan(&parentPath); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				r.log.Warn("Parent comment not found on creation attempt", zap.Int64("parent_id", *c.ParentID))
				return -1, fmt.Errorf("parent comment with id %d not found", *c.ParentID)
			}
			r.log.Error("Failed to scan parent path", zap.Error(err))
			return -1, fmt.Errorf("failed to scan parent path: %w", err)
		}
	}

	newPath := parentPath + c.PathID + "/"

	res, err := r.db.ExecWithRetry(ctx, retryStrategy, createQuery, c.ParentID, c.PathID, newPath, c.Comm, c.CreatedAt)
	if err != nil {
		r.log.Error("Failed to create comment in DB", zap.Error(err))
		return -1, fmt.Errorf("failed to create comment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		r.log.Warn("Failed to get last insert ID", zap.Error(err))
		return -1, nil
	}
	return id, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*models.Comment, error) {
	var comments models.Comment

	row, err := r.db.QueryRowWithRetry(ctx, retryStrategy, getCommentByIDQuery, id)
	if err != nil {
		r.log.Error("Failed to get comment by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get comment by ID: %w", err)
	}
	if err := row.Scan(&comments.ID, &comments.ParentID, &comments.PathID, &comments.Path, &comments.Comm, &comments.CreatedAt); err != nil {
		r.log.Error("Failed to scan comment by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get comment by ID: %w", err)
	}

	return &comments, nil
}

func (r *Repository) GetChildrenByPath(ctx context.Context, path string) ([]*models.Comment, error) {
	likePath := path + "%"
	rows, err := r.db.QueryWithRetry(ctx, retryStrategy, getChildrenByPathQuery, likePath)

	if err != nil {
		r.log.Error("Failed to get children by path", zap.String("path", path), zap.Error(err))
		return nil, fmt.Errorf("failed to get children by path: %w", err)
	}
	defer rows.Close()
	var comments []*models.Comment
	for rows.Next() {
		var comment models.Comment
		if err := rows.Scan(&comment.ID, &comment.ParentID, &comment.PathID, &comment.Path, &comment.Comm, &comment.CreatedAt); err != nil {
			r.log.Error("Failed to scan children by path", zap.String("path", path), zap.Error(err))
			return nil, fmt.Errorf("failed to get children by path: %w", err)
		}
		comments = append(comments, &comment)
	}
	return comments, nil
}

func (r *Repository) DeleteByPath(ctx context.Context, path string) error {

	likePath := path + "%"

	_, err := r.db.ExecWithRetry(ctx, retryStrategy, deleteCommentQuery, likePath)
	if err != nil {
		r.log.Error("Failed to delete comment by path", zap.String("path", path), zap.Error(err))
		return fmt.Errorf("failed to delete comment by path: %w", err)
	}

	return nil
}

func (r *Repository) GetTopLevelComments(ctx context.Context, limit, offset int, sortBy, sortOrder string) ([]*models.Comment, int, error) {
	var total int
	row, err := r.db.QueryRowWithRetry(ctx, retryStrategy, countTopLevelCommentsQuery)
	if err != nil {
		r.log.Error("Failed to get top level comments", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to get top level comments: %w", err)
	}
	if err := row.Scan(&total); err != nil {
		r.log.Error("Failed to count top level comments", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to count top level comments: %w", err)
	}

	allowedSorts := map[string]bool{"created_at": true, "id": true}
	if !allowedSorts[sortBy] {
		sortBy = "created_at"
	}
	sortOrder = strings.ToUpper(sortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "ASC"
	}

	query := fmt.Sprintf(`SELECT id,parent_id,path_id,path,comment,created_at FROM comments WHERE parent_id IS NULL ORDER BY %s %s LIMIT $1 OFFSET $2`, sortBy, sortOrder)

	rows, err := r.db.QueryWithRetry(ctx, retryStrategy, query, limit, offset)
	if err != nil {
		r.log.Error("Failed to get top level comments with pagination", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to get top level comments: %w", err)
	}
	defer rows.Close()

	var comments []*models.Comment
	for rows.Next() {
		var comment models.Comment
		if err := rows.Scan(&comment.ID, &comment.ParentID, &comment.PathID, &comment.Path, &comment.Comm, &comment.CreatedAt); err != nil {
			r.log.Error("Failed to scan top level comment", zap.Error(err))
			return nil, 0, fmt.Errorf("failed to scan top level comment: %w", err)
		}
		comments = append(comments, &comment)
	}
	return comments, total, nil
}

func (r *Repository) SearchByText(ctx context.Context, query string) ([]*models.Comment, error) {
	searchPattern := "%" + query + "%"

	rows, err := r.db.QueryWithRetry(ctx, retryStrategy, searchCommentsQuery, searchPattern)
	if err != nil {
		r.log.Error("Failed to search comments with LIKE", zap.String("query", query), zap.Error(err))
		return nil, fmt.Errorf("failed to search comments: %w", err)
	}
	defer rows.Close()

	var comments []*models.Comment
	for rows.Next() {
		var comment models.Comment
		if err := rows.Scan(&comment.ID, &comment.ParentID, &comment.PathID, &comment.Path, &comment.Comm, &comment.CreatedAt); err != nil {
			r.log.Error("Failed to scan searched comment", zap.Error(err))
			return nil, fmt.Errorf("failed to scan searched comment: %w", err)
		}
		comments = append(comments, &comment)
	}
	return comments, nil
}

func runMigrations(connStr string) error {
	migratePath := os.Getenv("MIGRATE_PATH")
	if migratePath == "" {
		migratePath = "./migrations"
	}
	absPath, err := filepath.Abs(migratePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	absPath = filepath.ToSlash(absPath)
	migrateUrl := fmt.Sprintf("file://%s", absPath)
	m, err := migrate.New(migrateUrl, connStr)
	if err != nil {
		return fmt.Errorf("start migrations error %v", err)
	}
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("migration up error: %v", err)
	}
	return nil
}
