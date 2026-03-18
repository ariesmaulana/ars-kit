package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/ariesmaulana/ars-kit/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|status|force <version>>")
		os.Exit(1)
	}

	cmd := os.Args[1]

	fmt.Println("[migrate] Loading config...")
	cfg, err := config.InitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[migrate] config error: %v\n", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf(
		"pgx5://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	fmt.Printf("[migrate] Connecting to database: %s:%s/%s\n", cfg.DBHost, cfg.DBPort, cfg.DBName)

	migrationsPath := "file://database/migrations"
	fmt.Printf("[migrate] Migrations path: database/migrations\n")

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[migrate] failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	switch cmd {
	case "up":
		fmt.Println("[migrate] Applying pending migrations...")
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("[migrate] No pending migrations.")
				return
			}
			version, dirty, _ := m.Version()
			fmt.Fprintf(os.Stderr, "[migrate] migration failed at version %d (dirty=%v): %v\n", version, dirty, err)
			os.Exit(1)
		}
		version, _, _ := m.Version()
		fmt.Printf("[migrate] ✓ Done — migrated to version %d\n", version)

	case "status":
		version, dirty, err := m.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			fmt.Fprintf(os.Stderr, "[migrate] failed to get version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[migrate] Current schema version: %d\n", version)
		fmt.Printf("[migrate] Dirty: %v\n", dirty)

	case "force":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "[migrate] usage: migrate force <version>")
			os.Exit(1)
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "[migrate] invalid version: %s\n", os.Args[2])
			os.Exit(1)
		}
		if err := m.Force(v); err != nil {
			fmt.Fprintf(os.Stderr, "[migrate] force failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[migrate] ✓ Forced version to %d (dirty flag cleared)\n", v)

	default:
		fmt.Fprintf(os.Stderr, "[migrate] unknown command: %s (use up, status, or force <version>)\n", cmd)
		os.Exit(1)
	}
}
