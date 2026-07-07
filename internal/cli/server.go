package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/platform/server"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
	"github.com/jesseedcp/devflow-agent/internal/platform/store/mysql"
)

// runServer starts the platform API server. It reads the database DSN from
// --database-dsn or DEVFLOW_DATABASE_DSN, opens MySQL, runs migrations, seeds
// the dev-mode identity, and serves the JSON API. It never prints the DSN.
func runServer(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("devflow server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	root := fs.String("root", "", "default artifact root (defaults to current directory)")
	dsn := fs.String("database-dsn", "", "MySQL DSN (defaults to DEVFLOW_DATABASE_DSN)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dsnVal := strings.TrimSpace(*dsn)
	if dsnVal == "" {
		dsnVal = os.Getenv("DEVFLOW_DATABASE_DSN")
	}
	if dsnVal == "" {
		return errors.New("database dsn is required: set --database-dsn or DEVFLOW_DATABASE_DSN")
	}

	rootVal := strings.TrimSpace(*root)
	if rootVal == "" {
		rootVal, _ = os.Getwd()
	}

	email := os.Getenv("DEVFLOW_DEV_USER_EMAIL")
	if email == "" {
		email = "admin@example.com"
	}
	role := store.RoleAdmin
	if v := os.Getenv("DEVFLOW_DEV_USER_ROLE"); v != "" {
		role = store.ParseRole(v)
	}

	logger := log.New(stderr, "devflow-server: ", log.LstdFlags)
	logger.Printf("starting platform server on %s (artifact root: %s)", *addr, rootVal)

	openCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := mysql.Open(openCtx, dsnVal)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	if err := st.Migrate(context.Background()); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	const devUserID = "devlocal"
	if err := st.UpsertUser(openCtx, store.User{ID: devUserID, Email: email, DisplayName: email}); err != nil {
		return fmt.Errorf("seed dev user: %w", err)
	}

	cfg := server.Config{
		Addr:         *addr,
		ArtifactRoot: rootVal,
		DevUserEmail: email,
		DevUserRole:  role,
		DevUserID:    devUserID,
		Logger:       logger,
	}
	srv := server.New(cfg, st)
	return srv.Start()
}