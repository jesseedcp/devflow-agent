package memory

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	memoryCandidatesFile     = "memory-candidates.md"
	fileAttributeReparseMask = 0x400
)

var demandIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Result struct {
	DemandID string
	Path     string
	Snippet  string
	Source   Source
}

type Store struct {
	root string
}

func NewStore(root string) Store {
	return Store{root: root}
}

func (s Store) Search(query string) ([]Result, error) {
	if s.root == "" {
		return nil, fmt.Errorf("store root is required")
	}

	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, fmt.Errorf("query is required")
	}

	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return nil, fmt.Errorf("resolve store root: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve store root: %w", err)
	}

	devflowPath := filepath.Join(rootAbs, ".devflow")
	expectedDevflow := filepath.Join(rootResolved, ".devflow")
	exists, err := ensureSafePath(devflowPath, expectedDevflow)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	base := filepath.Join(devflowPath, "demands")
	expectedBase := filepath.Join(expectedDevflow, "demands")
	exists, err = ensureSafePath(base, expectedBase)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("read demands directory: %w", err)
	}

	results := make([]Result, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		demandID := entry.Name()
		if !demandIDPattern.MatchString(demandID) {
			continue
		}

		demandDir := filepath.Join(base, demandID)
		expectedDemand := filepath.Join(expectedBase, demandID)
		unsafe, err := isUnsafeDirectory(demandDir, expectedDemand)
		if err != nil {
			return nil, fmt.Errorf("inspect demand directory %s: %w", demandID, err)
		}
		if unsafe {
			continue
		}

		path := filepath.Join(demandDir, memoryCandidatesFile)
		expectedCandidate := filepath.Join(expectedDemand, memoryCandidatesFile)
		data, err := readCandidateFile(path, expectedCandidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read memory candidates for %s (%s): %w", demandID, path, err)
		}

		text := string(data)
		if matchesAll(strings.ToLower(text), terms) {
			results = append(results, Result{
				DemandID: demandID,
				Path:     path,
				Snippet:  firstLine(text),
				Source:   SourceCandidate,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DemandID < results[j].DemandID
	})

	return results, nil
}

func matchesAll(text string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}
	return ""
}

func readCandidateFile(path, expectedResolved string) (data []byte, err error) {
	if _, err := inspectCandidateFile(path, expectedResolved); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open candidate file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			data = nil
			if err != nil {
				err = fmt.Errorf("%w (and close candidate file: %v)", err, closeErr)
				return
			}
			err = fmt.Errorf("close candidate file: %w", closeErr)
		}
	}()

	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened candidate file: %w", err)
	}

	currentInfo, err := inspectCandidateFile(path, expectedResolved)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(openedInfo, currentInfo) {
		return nil, fmt.Errorf("candidate changed during read")
	}

	data, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read candidate file: %w", err)
	}
	return data, nil
}

func inspectCandidateFile(path, expectedResolved string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe candidate path %q: symlink not allowed", path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("candidate path %q is a directory", path)
	}

	reparsePoint, err := hasReparsePoint(info)
	if err != nil {
		return nil, fmt.Errorf("inspect candidate path reparse point: %w", err)
	}
	if reparsePoint {
		return nil, fmt.Errorf("unsafe candidate path %q: reparse point not allowed", path)
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolve candidate path: %w", err)
	}
	if !samePath(resolvedPath, expectedResolved) {
		return nil, fmt.Errorf("unsafe candidate path %q resolves outside %q", path, expectedResolved)
	}

	return info, nil
}

func ensureSafePath(path, expectedResolved string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect memory path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("unsafe memory path %q: symlink not allowed", path)
	}

	reparsePoint, err := hasReparsePoint(info)
	if err != nil {
		return false, fmt.Errorf("inspect memory path reparse point: %w", err)
	}
	if reparsePoint {
		return false, fmt.Errorf("unsafe memory path %q: reparse point not allowed", path)
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, fmt.Errorf("resolve memory path: %w", err)
	}
	if !samePath(resolvedPath, expectedResolved) {
		return false, fmt.Errorf("unsafe memory path %q resolves outside %q", path, expectedResolved)
	}

	return true, nil
}

func isUnsafeDirectory(path, expectedResolved string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}

	reparsePoint, err := hasReparsePoint(info)
	if err != nil {
		return false, err
	}
	if reparsePoint {
		return true, nil
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, fmt.Errorf("resolve symlinks: %w", err)
	}

	return !samePath(resolvedPath, expectedResolved), nil
}

func hasReparsePoint(info os.FileInfo) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, nil
	}

	sys := info.Sys()
	if sys == nil {
		return false, nil
	}

	value := reflect.ValueOf(sys)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return false, nil
	}

	elem := value.Elem()
	if !elem.IsValid() {
		return false, nil
	}

	attributes := elem.FieldByName("FileAttributes")
	if !attributes.IsValid() {
		return false, nil
	}
	if !attributes.CanUint() {
		return false, fmt.Errorf("unexpected FileAttributes kind %s", attributes.Kind())
	}

	return attributes.Uint()&fileAttributeReparseMask != 0, nil
}

func samePath(left, right string) bool {
	left = canonicalComparePath(left)
	right = canonicalComparePath(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func canonicalComparePath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return path
}

type stableMemoryPaths struct {
	devflowPath     string
	expectedDevflow string
	memDir          string
	expectedMemDir  string
}

func (s Store) stableMemoryPaths() (stableMemoryPaths, error) {
	if s.root == "" {
		return stableMemoryPaths{}, fmt.Errorf("store root is required")
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return stableMemoryPaths{}, fmt.Errorf("resolve store root: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return stableMemoryPaths{}, fmt.Errorf("resolve store root: %w", err)
	}
	devflowPath := filepath.Join(rootAbs, ".devflow")
	expectedDevflow := filepath.Join(rootResolved, ".devflow")
	memDir := filepath.Join(devflowPath, "memory")
	expectedMemDir := filepath.Join(expectedDevflow, "memory")
	return stableMemoryPaths{
		devflowPath:     devflowPath,
		expectedDevflow: expectedDevflow,
		memDir:          memDir,
		expectedMemDir:  expectedMemDir,
	}, nil
}

func (s Store) SearchStable(query string) ([]Result, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, fmt.Errorf("query is required")
	}

	paths, err := s.stableMemoryPaths()
	if err != nil {
		return nil, err
	}
	exists, err := ensureSafePath(paths.memDir, paths.expectedMemDir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	entries, err := os.ReadDir(paths.memDir)
	if err != nil {
		return nil, fmt.Errorf("read stable memory directory: %w", err)
	}
	results := make([]Result, 0)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "MEMORY.md" || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(paths.memDir, entry.Name())
		expectedPath := filepath.Join(paths.expectedMemDir, entry.Name())
		data, err := readStableMemoryFile(path, expectedPath)
		if err != nil {
			return nil, fmt.Errorf("read stable memory %s: %w", entry.Name(), err)
		}
		text := string(data)
		if !matchesAll(strings.ToLower(text), terms) {
			continue
		}
		results = append(results, Result{
			Path:    path,
			Snippet: stableSnippet(text),
			Source:  SourceStable,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return results, nil
}

func stableSnippet(text string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		}
	}
	return firstLine(text)
}

func readStableMemoryFile(path, expectedResolved string) (data []byte, err error) {
	if _, err := inspectStableMemoryFile(path, expectedResolved); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stable memory file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			data = nil
			if err != nil {
				err = fmt.Errorf("%w (and close stable memory file: %v)", err, closeErr)
				return
			}
			err = fmt.Errorf("close stable memory file: %w", closeErr)
		}
	}()

	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened stable memory file: %w", err)
	}

	currentInfo, err := inspectStableMemoryFile(path, expectedResolved)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(openedInfo, currentInfo) {
		return nil, fmt.Errorf("stable memory changed during read")
	}

	data, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read stable memory file: %w", err)
	}
	return data, nil
}

func inspectStableMemoryFile(path, expectedResolved string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe stable memory path %q: symlink not allowed", path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("stable memory path %q is a directory", path)
	}

	reparsePoint, err := hasReparsePoint(info)
	if err != nil {
		return nil, fmt.Errorf("inspect stable memory path reparse point: %w", err)
	}
	if reparsePoint {
		return nil, fmt.Errorf("unsafe stable memory path %q: reparse point not allowed", path)
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolve stable memory path: %w", err)
	}
	if !samePath(resolvedPath, expectedResolved) {
		return nil, fmt.Errorf("unsafe stable memory path %q resolves outside %q", path, expectedResolved)
	}

	return info, nil
}
