package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var migrationFS embed.FS

// Up applies all *.up.sql files in sql/ in lexical order.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	var ups []string
	err := fs.WalkDir(migrationFS, "sql", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".up.sql") {
			ups = append(ups, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk migrations: %w", err)
	}
	sort.Strings(ups)
	for _, path := range ups {
		body, err := migrationFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		sql := strings.TrimSpace(string(body))
		if sql == "" {
			continue
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("exec %s: %w", path, err)
		}
	}
	return nil
}
