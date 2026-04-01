package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"github.com/ariesmaulana/ars-kit/src/app/user"
)

// Domain represents a migration domain with its embedded SQL files.
type Domain struct {
	Name string
	FS   fs.FS
}

// All contains every domain's migrations in dependency order.
var All = []Domain{
	{Name: "user", FS: user.Migrations},
}

// UserOnly is a convenience slice for running only user-domain migrations.
var UserOnly = All[:1]

// Run applies goose migrations for the given domains.
// The db connection's search_path should already target the desired schema.
func Run(db *sql.DB, schema string, domains []Domain) error {
	for _, d := range domains {
		sqlFS, err := fs.Sub(d.FS, "sql")
		if err != nil {
			return fmt.Errorf("migrate %s: sub fs: %w", d.Name, err)
		}
		provider, err := goose.NewProvider(goose.DialectPostgres, db, sqlFS)
		if err != nil {
			return fmt.Errorf("migrate %s: create provider: %w", d.Name, err)
		}
		if _, err := provider.Up(context.Background()); err != nil {
			return fmt.Errorf("migrate %s: %w", d.Name, err)
		}
	}
	return nil
}
