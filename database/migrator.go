package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"github.com/ariesmaulana/ars-kit/src/app/permission"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/app/workflow"
)

// Domain represents a migration domain with its embedded SQL files.
//
// TableName is the goose version-history table for the domain. Every goose
// provider shares the same "goose_db_version" table by default, so a migration
// in one domain looks already-applied (or out-of-order) when a later domain
// uses the same or an older version number. Because domains are developed in
// parallel and each domain's Up applies all of its pending migrations at once,
// domains must track history independently: each gets its own table.
type Domain struct {
	Name      string
	TableName string
	FS        fs.FS
}

// All contains every domain's migrations in dependency order.
var All = []Domain{
	{Name: "user", TableName: "goose_db_version_user", FS: user.Migrations},
	{Name: "permission", TableName: "goose_db_version_permission", FS: permission.Migrations},
	{Name: "workflow", TableName: "goose_db_version_workflow", FS: workflow.Migrations},
}

// UserOnly is a convenience slice for running only user-domain migrations.
var UserOnly = All[:1]

// PermissionOnly is a convenience slice for running only permission-domain migrations.
var PermissionOnly = All[1:2]

// WorkflowOnly is a convenience slice for running only workflow-domain migrations.
var WorkflowOnly = All[2:3]

// NewProvider builds a goose provider for a domain bound to the domain's own
// version-history table.
func NewProvider(db *sql.DB, d Domain) (*goose.Provider, error) {
	sqlFS, err := fs.Sub(d.FS, "sql")
	if err != nil {
		return nil, fmt.Errorf("migrate %s: sub fs: %w", d.Name, err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sqlFS, goose.WithTableName(d.TableName))
	if err != nil {
		return nil, fmt.Errorf("migrate %s: create provider: %w", d.Name, err)
	}
	return provider, nil
}

// Run applies goose migrations for the given domains.
// The db connection's search_path should already target the desired schema.
func Run(db *sql.DB, schema string, domains []Domain) error {
	for _, d := range domains {
		provider, err := NewProvider(db, d)
		if err != nil {
			return fmt.Errorf("migrate %s: %w", d.Name, err)
		}
		if _, err := provider.Up(context.Background()); err != nil {
			return fmt.Errorf("migrate %s: %w", d.Name, err)
		}
	}
	return nil
}
