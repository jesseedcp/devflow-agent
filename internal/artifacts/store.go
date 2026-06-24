package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/templates"
)

var demandIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Store struct {
	root string
}

func NewStore(root string) Store {
	return Store{root: root}
}

func (s Store) DemandDir(id string) string {
	return filepath.Join(s.root, ".devflow", "demands", id)
}

func (s Store) CreateDemand(demand Demand) error {
	now := time.Now().UTC()
	if demand.ID == "" {
		return fmt.Errorf("demand id is required")
	}
	if demand.Title == "" {
		return fmt.Errorf("demand title is required")
	}
	if demand.CreatedAt.IsZero() {
		demand.CreatedAt = now
	}
	demand.UpdatedAt = now

	paths, err := s.prepareDemandPaths(demand.ID, true)
	if err != nil {
		return err
	}
	if exists, err := demandWorkspaceExists(paths.demandDir); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("demand %s already exists", demand.ID)
	}

	tempDir, err := os.MkdirTemp(paths.baseDir, demand.ID+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp demand directory: %w", err)
	}

	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if err := os.Chmod(tempDir, 0o755); err != nil {
		return fmt.Errorf("chmod temp demand directory: %w", err)
	}
	if err := writeJSON(filepath.Join(tempDir, DemandFile), demand); err != nil {
		return fmt.Errorf("write demand file: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, RequirementsFile), templates.Requirements(demand.Title, demand.Description)); err != nil {
		return fmt.Errorf("write requirements template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, PlanFile), templates.Plan(demand.Title)); err != nil {
		return fmt.Errorf("write plan template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, ProgressFile), "# Progress\n\n"); err != nil {
		return fmt.Errorf("write progress template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, VerificationFile), templates.Verification(demand.Title)); err != nil {
		return fmt.Errorf("write verification template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, CloseoutFile), templates.Closeout(demand.Title)); err != nil {
		return fmt.Errorf("write closeout template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, MemoryCandidatesFile), templates.MemoryCandidates(demand.Title)); err != nil {
		return fmt.Errorf("write memory candidates template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, EventsFile), ""); err != nil {
		return fmt.Errorf("create events file: %w", err)
	}
	if err := appendEventFile(filepath.Join(tempDir, EventsFile), Event{
		Time:    now,
		Type:    "demand.created",
		Message: "demand workspace created",
	}); err != nil {
		return fmt.Errorf("append demand created event: %w", err)
	}
	if err := ensureResolvedPath(paths.devflowDir, paths.expectedDevflowResolved); err != nil {
		return err
	}
	if err := ensureResolvedPath(paths.baseDir, paths.expectedBaseResolved); err != nil {
		return err
	}
	if err := ensureDemandDirSafe(paths.demandDir, paths.expectedDemandResolved); err != nil {
		return err
	}
	if exists, err := demandWorkspaceExists(paths.demandDir); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("demand %s already exists", demand.ID)
	}
	if err := os.Rename(tempDir, paths.demandDir); err != nil {
		if exists, statErr := demandWorkspaceExists(paths.demandDir); statErr == nil && exists {
			return fmt.Errorf("demand %s already exists", demand.ID)
		}
		return fmt.Errorf("rename demand workspace: %w", err)
	}
	cleanupTemp = false

	return nil
}

func (s Store) LoadDemand(id string) (Demand, error) {
	paths, err := s.prepareDemandPaths(id, false)
	if err != nil {
		return Demand{}, err
	}

	path := filepath.Join(paths.demandDir, DemandFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return Demand{}, fmt.Errorf("read demand: %w", err)
	}

	var demand Demand
	if err := json.Unmarshal(data, &demand); err != nil {
		return Demand{}, fmt.Errorf("decode demand: %w", err)
	}

	return demand, nil
}

func (s Store) SaveDemand(demand Demand) error {
	paths, err := s.prepareDemandPaths(demand.ID, false)
	if err != nil {
		return err
	}

	demand.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(filepath.Join(paths.demandDir, DemandFile), demand); err != nil {
		return fmt.Errorf("write demand file: %w", err)
	}
	return nil
}

func (s Store) AppendEvent(id string, event Event) error {
	paths, err := s.prepareDemandPaths(id, false)
	if err != nil {
		return err
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	return appendEventFile(filepath.Join(paths.demandDir, EventsFile), event)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write json: %w", err)
	}

	return nil
}

func writeTextFile(path, contents string) error {
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func writeJSONAtomic(path string, value any) (err error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')

	tempFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp json file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err = tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("write json: %w", err)
	}
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("sync json file: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("close json file: %w", err)
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename json file: %w", err)
	}

	return nil
}

func appendEventFile(path string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events log: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync events log: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close events log: %w", err)
	}

	return nil
}

type demandPaths struct {
	devflowDir              string
	baseDir                 string
	demandDir               string
	expectedDevflowResolved string
	expectedBaseResolved    string
	expectedDemandResolved  string
}

func (s Store) prepareDemandPaths(id string, createBase bool) (demandPaths, error) {
	if s.root == "" {
		return demandPaths{}, fmt.Errorf("store root is required")
	}
	if err := validateDemandID(id); err != nil {
		return demandPaths{}, err
	}

	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return demandPaths{}, fmt.Errorf("resolve store root: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	rootResolved, err := evalSymlinksIfExists(rootAbs)
	if err != nil {
		return demandPaths{}, fmt.Errorf("resolve store root: %w", err)
	}

	devflowDir := filepath.Join(rootAbs, ".devflow")
	expectedDevflowResolved := filepath.Join(rootResolved, ".devflow")
	baseDir := filepath.Join(rootAbs, ".devflow", "demands")
	expectedBaseResolved := filepath.Join(rootResolved, ".devflow", "demands")

	if err := ensureResolvedPath(devflowDir, expectedDevflowResolved); err != nil {
		return demandPaths{}, err
	}

	if createBase {
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			return demandPaths{}, fmt.Errorf("create demand base directory: %w", err)
		}
	}
	if err := ensureResolvedPath(baseDir, expectedBaseResolved); err != nil {
		return demandPaths{}, err
	}

	demandDir := filepath.Join(baseDir, id)
	if err := ensureDemandDirSafe(demandDir, filepath.Join(expectedBaseResolved, id)); err != nil {
		return demandPaths{}, err
	}

	return demandPaths{
		devflowDir:              devflowDir,
		baseDir:                 baseDir,
		demandDir:               demandDir,
		expectedDevflowResolved: expectedDevflowResolved,
		expectedBaseResolved:    expectedBaseResolved,
		expectedDemandResolved:  filepath.Join(expectedBaseResolved, id),
	}, nil
}

func validateDemandID(id string) error {
	if !demandIDPattern.MatchString(id) {
		return fmt.Errorf("invalid demand id %q", id)
	}
	return nil
}

func ensureResolvedPath(path, expectedResolved string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect demand path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe demand path %q: symlink not allowed", path)
	}
	if reparsePoint, err := pathIsReparsePoint(path); err != nil {
		return fmt.Errorf("inspect demand path reparse point: %w", err)
	} else if reparsePoint {
		return fmt.Errorf("unsafe demand path %q: reparse point not allowed", path)
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve demand path: %w", err)
	}
	if !samePath(resolvedPath, expectedResolved) {
		return fmt.Errorf("unsafe demand path %q resolves outside %q", path, expectedResolved)
	}

	return nil
}

func ensureDemandDirSafe(path, expectedResolved string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect demand directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe demand directory %q: symlink not allowed", path)
	}
	if reparsePoint, err := pathIsReparsePoint(path); err != nil {
		return fmt.Errorf("inspect demand directory reparse point: %w", err)
	} else if reparsePoint {
		return fmt.Errorf("unsafe demand directory %q: reparse point not allowed", path)
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve demand directory: %w", err)
	}
	if !samePath(resolvedPath, expectedResolved) {
		return fmt.Errorf("unsafe demand directory %q resolves outside demand root", path)
	}

	return nil
}

func demandWorkspaceExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("inspect demand workspace: %w", err)
}

func evalSymlinksIfExists(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	return resolved, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
