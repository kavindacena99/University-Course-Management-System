package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	direction := flag.String("direction", "up", "migration direction: up, down, version, or force")
	steps := flag.Int("steps", 0, "number of migration steps; 0 means all")
	path := flag.String("path", "migrations", "path to migration files")
	forceVersion := flag.Int("force-version", -1, "version to force")
	flag.Parse()
	_ = godotenv.Load()
	dsn, err := databaseURL()
	if err != nil {
		return err
	}
	migrator, err := migrate.New("file://"+filepath.ToSlash(*path), dsn)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()
	switch strings.ToLower(*direction) {
	case "up":
		if *steps > 0 {
			err = migrator.Steps(*steps)
		} else {
			err = migrator.Up()
		}
	case "down":
		if *steps > 0 {
			err = migrator.Steps(-*steps)
		} else {
			err = migrator.Down()
		}
	case "version":
		return printVersion(migrator)
	case "force":
		if *forceVersion < 0 {
			return fmt.Errorf("-force-version is required for force")
		}
		err = migrator.Force(*forceVersion)
	default:
		return fmt.Errorf("unsupported direction %q", *direction)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return printVersion(migrator)
}

func printVersion(migrator *migrate.Migrate) error {
	version, dirty, err := migrator.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		fmt.Println("version: none")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read version: %w", err)
	}
	fmt.Printf("version: %d (dirty=%t)\n", version, dirty)
	return nil
}

func databaseURL() (string, error) {
	host := strings.TrimSpace(os.Getenv("DB_HOST"))
	portText := strings.TrimSpace(os.Getenv("DB_PORT"))
	name := strings.TrimSpace(os.Getenv("DB_NAME"))
	user := strings.TrimSpace(os.Getenv("DB_USER"))
	password := os.Getenv("DB_PASSWORD")
	sslMode := strings.TrimSpace(os.Getenv("DB_SSLMODE"))
	if host == "" || name == "" || user == "" || password == "" {
		return "", fmt.Errorf("DB_HOST, DB_NAME, DB_USER, and DB_PASSWORD must be configured")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("DB_PORT must be between 1 and 65535")
	}
	if sslMode == "" {
		sslMode = "disable"
	}
	dsn := &url.URL{
		Scheme: "pgx5", User: url.UserPassword(user, password),
		Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: "/" + name,
	}
	query := dsn.Query()
	query.Set("sslmode", sslMode)
	query.Set("x-multi-statement", "true")
	dsn.RawQuery = query.Encode()
	return dsn.String(), nil
}
