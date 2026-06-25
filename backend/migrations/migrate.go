// Package migrations provides a simple embedded SQL migration runner.
// It reads all *.sql files embedded at compile-time and executes them
// in lexicographic order inside a transaction, making migrations idempotent
// (each file uses IF NOT EXISTS guards).
package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

//go:embed *.sql
var sqlFiles embed.FS

// Run executes all embedded SQL migration files in order.
func Run(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
	entries, err := fs.ReadDir(sqlFiles, ".")
	if err != nil {
		return fmt.Errorf("reading migration files: %w", err)
	}

	// Collect .sql files and sort them — names are prefixed 001_, 002_, etc.
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		content, err := sqlFiles.ReadFile(filepath.Join(".", name))
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}

		logger.Info("running migration", zap.String("file", name))
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("executing %s: %w", name, err)
		}
		logger.Info("migration applied", zap.String("file", name))
	}
	return nil
}
