package database

import (
	"embed"
	"fmt"
	"io/fs"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations executes all pending migrations using golang-migrate library
func (db *DB) runMigrations() error {
	// Get the migrations subdirectory from the embedded filesystem
	migrationsDir, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations subdirectory: %w", err)
	}

	// Create a source driver from the embedded filesystem
	sourceDriver, err := iofs.New(migrationsDir, ".")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	// Create a database driver for SQLite
	dbDriver, err := sqlite3.WithInstance(db.conn, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create the migrate instance
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d, manual intervention required", version)
	}

	// Run migrations
	log.Printf("Current database version: %d", version)
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Printf("Database schema is up to date")
	} else {
		newVersion, _, _ := m.Version()
		log.Printf("Migrations applied successfully, new version: %d", newVersion)
	}

	return nil
}
