package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/ariesmaulana/ars-kit/database"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|status|create> [domain] [name]")
		os.Exit(1)
	}

	cmd := os.Args[1]

	if cmd == "create" {
		runCreate()
		return
	}

	fmt.Println("[migrate] Loading config...")
	cfg, err := config.InitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[migrate] config error: %v\n", err)
		os.Exit(1)
	}

	schema := cfg.DBSchema
	if schema == "" {
		schema = "public"
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName, schema,
	)

	fmt.Printf("[migrate] Connecting to database: %s:%s/%s (schema: %s)\n", cfg.DBHost, cfg.DBPort, cfg.DBName, schema)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[migrate] failed to open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Determine which domains to migrate
	domains := database.All
	if len(os.Args) >= 3 {
		domainName := os.Args[2]
		domains = filterDomains(domainName)
		if domains == nil {
			fmt.Fprintf(os.Stderr, "[migrate] unknown domain: %s\n", domainName)
			os.Exit(1)
		}
		fmt.Printf("[migrate] Scoped to domain: %s\n", domainName)
	}

	ctx := context.Background()

	switch cmd {
	case "up":
		fmt.Println("[migrate] Applying pending migrations...")
		if err := database.Run(db, schema, domains); err != nil {
			fmt.Fprintf(os.Stderr, "[migrate] migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[migrate] Done.")

	case "down":
		for _, d := range domains {
			provider, err := newProvider(db, d)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[migrate] %v\n", err)
				os.Exit(1)
			}
			if _, err := provider.Down(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "[migrate] down failed for %s: %v\n", d.Name, err)
				os.Exit(1)
			}
		}
		fmt.Println("[migrate] Rolled back one version.")

	case "status":
		for _, d := range domains {
			provider, err := newProvider(db, d)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[migrate] %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("\n[migrate] Domain: %s\n", d.Name)
			statuses, err := provider.Status(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[migrate] status failed for %s: %v\n", d.Name, err)
				os.Exit(1)
			}
			for _, s := range statuses {
				state := "Pending"
				if s.State == goose.StateApplied {
					state = fmt.Sprintf("Applied  %s", s.AppliedAt.Format("2006-01-02 15:04:05"))
				}
				fmt.Printf("    %-14d %s\n", s.Source.Version, state)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "[migrate] unknown command: %s (use up, down, or status)\n", cmd)
		os.Exit(1)
	}
}

func newProvider(db *sql.DB, d database.Domain) (*goose.Provider, error) {
	sqlFS, err := fs.Sub(d.FS, "sql")
	if err != nil {
		return nil, fmt.Errorf("domain %s: sub fs: %w", d.Name, err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sqlFS)
	if err != nil {
		return nil, fmt.Errorf("domain %s: create provider: %w", d.Name, err)
	}
	return provider, nil
}

func filterDomains(name string) []database.Domain {
	for _, d := range database.All {
		if d.Name == name {
			return []database.Domain{d}
		}
	}
	return nil
}

func runCreate() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: migrate create <domain> <name>")
		os.Exit(1)
	}

	domain := os.Args[2]
	name := os.Args[3]

	dir := fmt.Sprintf("src/app/%s/sql", domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[migrate] failed to create directory %s: %v\n", dir, err)
		os.Exit(1)
	}

	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s/%s_%s.sql", dir, ts, name)

	content := "-- +goose Up\n\n-- +goose Down\n"
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[migrate] failed to create file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[migrate] Created: %s\n", filename)
}
