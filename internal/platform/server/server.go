package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

// Store is the persistence port the platform server depends on. The MySQL
// implementation satisfies it; tests use an in-memory fake.
type Store interface {
	Ping(ctx context.Context) error
	Migrate(ctx context.Context) error
	Close() error
	UpsertWorkspace(ctx context.Context, w store.Workspace) error
	ListWorkspaces(ctx context.Context) ([]store.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (store.Workspace, error)
	UpsertUser(ctx context.Context, u store.User) error
	GetUserByEmail(ctx context.Context, email string) (store.User, error)
	UpsertWorkspaceMember(ctx context.Context, m store.WorkspaceMember) error
	ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]store.WorkspaceMember, error)
	AppendAudit(ctx context.Context, ev store.AuditEvent) error
	ListAudit(ctx context.Context, workspaceID string) ([]store.AuditEvent, error)
}

// Config holds platform server startup configuration.
type Config struct {
	Addr         string
	ArtifactRoot string
	DevUserEmail string
	DevUserRole  store.Role
	DevUserID    string
	Logger       *log.Logger
}

// Server is the Devflow platform HTTP server.
type Server struct {
	cfg   Config
	store Store
	mux   *http.ServeMux
}

// New constructs a Server and registers its routes.
func New(cfg Config, st Store) *Server {
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stderr, "devflow-server: ", log.LstdFlags)
	}
	s := &Server{cfg: cfg, store: st, mux: http.NewServeMux()}
	s.registerRoutes()
	return s
}

// Handler returns the routed HTTP handler, primarily for testing.
func (s *Server) Handler() http.Handler { return s.mux }

// Start listens on the configured address and serves until interrupted.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	s.cfg.Logger.Printf("listening on %s", ln.Addr())
	srv := &http.Server{Handler: s.mux}
	return srv.Serve(ln)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, api.ErrorResponse{Error: msg})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func newID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "id-" + hex.EncodeToString(buf[:8])
	}
	return hex.EncodeToString(buf[:])
}