// Command migrate is an operator-facing wrapper around the same
// golang-migrate library that the backend server uses at startup
// (see internal/database.RunMigrationsWithOptions). It exists so that
// CI, the Helm chart Job, and human operators can apply / roll back
// migrations without booting the whole HTTP server.
//
// Usage:
//
//   migrate -dsn <url> -action up
//   migrate -dsn <url> -action down -steps 1
//   migrate -dsn <url> -action status
//   migrate -dsn <url> -action force -version 7
//
// DSN formats:
//   postgres://user:pass@host:5432/db?sslmode=disable
//   sqlite3:///absolute/path/to.db   (uses migrations/sqlite/)
//
// When DSN is empty, $AIKBS_DB_URL is read. Migrations are sourced
// from migrations/versioned (or migrations/sqlite for sqlite DSNs).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("AIKBS_DB_URL"), "Database DSN (or set AIKBS_DB_URL)")
	action := flag.String("action", "status", "up | down | status | force")
	steps := flag.Int("steps", 1, "Number of migrations to apply (down)")
	version := flag.Int("version", -1, "Version to force (force action)")
	migrationsPath := flag.String("migrations", "", "Override migrations dir (file://...). Defaults to migrations/versioned")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -dsn or AIKBS_DB_URL is required")
		os.Exit(2)
	}

	if *migrationsPath == "" {
		if strings.HasPrefix(*dsn, "sqlite3://") || strings.HasPrefix(*dsn, "sqlite://") {
			*migrationsPath = "file://migrations/sqlite"
		} else {
			*migrationsPath = "file://migrations/versioned"
		}
	}

	m, err := migrate.New(*migrationsPath, *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create migrator: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Fprintf(os.Stderr, "WARN: source close error: %v\n", srcErr)
		}
		if dbErr != nil {
			fmt.Fprintf(os.Stderr, "WARN: db close error: %v\n", dbErr)
		}
	}()

	switch *action {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "ERROR: migrate up: %v\n", err)
			os.Exit(1)
		}
		printVersion(m, "after up")
	case "down":
		if err := m.Steps(-*steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "ERROR: migrate down: %v\n", err)
			os.Exit(1)
		}
		printVersion(m, "after down")
	case "status":
		printVersion(m, "current")
	case "force":
		if *version < 0 {
			fmt.Fprintln(os.Stderr, "ERROR: -action force requires -version N")
			os.Exit(2)
		}
		if err := m.Force(*version); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: migrate force: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "OK: forced version to %d\n", *version)
		printVersion(m, "after force")
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown action %q (want up|down|status|force)\n", *action)
		os.Exit(2)
	}
}

func printVersion(m *migrate.Migrate, label string) {
	v, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		fmt.Printf("%s: <no version applied>\n", label)
		return
	case err != nil:
		fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", label, err)
		os.Exit(1)
	}
	fmt.Printf("%s: version=%d dirty=%t\n", label, v, dirty)
	_ = strconv.Itoa // keep import in case build tags trim
}