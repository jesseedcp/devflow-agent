package artifacts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/templates"
)

var demandIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Store struct {
	root string
}

const demandLockFile = ".confirm.lock"

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
	if err := writeTextFile(filepath.Join(tempDir, IntakeFile), templates.Intake(demand.Title, demand.Source)); err != nil {
		return fmt.Errorf("write intake template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, ContextFile), templates.Context(demand.Title)); err != nil {
		return fmt.Errorf("write context template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, CodemapFile), "# Codemap Context: "+demand.Title+"\n\n"); err != nil {
		return fmt.Errorf("write codemap template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, PlanContextFile), "# Plan Context: "+demand.Title+"\n\n"); err != nil {
		return fmt.Errorf("write plan context template: %w", err)
	}
	if err := writeTextFile(filepath.Join(tempDir, ChangeScopeFile), "# Change Scope: "+demand.Title+"\n\n## Source Files\n\n## Test Files\n\n## Out Of Scope\n\n"); err != nil {
		return fmt.Errorf("write change scope template: %w", err)
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
func (s Store) ReadEvents(demandID string) ([]Event, error) {
	paths, err := s.prepareDemandPaths(demandID, false)
	if err != nil {
		return nil, err
	}
	events, err := readEventsLogRecoverTrailing(filepath.Join(paths.demandDir, EventsFile))
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s Store) AppendToArtifact(id, name, content string) error {
	paths, err := s.prepareDemandPaths(id, false)
	if err != nil {
		return err
	}
	if err := validateAppendableArtifactName(name); err != nil {
		return err
	}

	path := filepath.Join(paths.demandDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open artifact %s: %w", name, err)
	}

	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return fmt.Errorf("append artifact %s: %w", name, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync artifact %s: %w", name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close artifact %s: %w", name, err)
	}

	return nil
}

func (s Store) WriteArtifact(id, name, content string) error {
	paths, err := s.prepareDemandPaths(id, false)
	if err != nil {
		return err
	}
	if err := validateAppendableArtifactName(name); err != nil {
		return err
	}
	if exists, err := demandWorkspaceExists(paths.demandDir); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("demand %s does not exist", id)
	}

	path := filepath.Join(paths.demandDir, name)
	if err := writeTextAtomic(path, content); err != nil {
		return fmt.Errorf("write artifact %s: %w", name, err)
	}

	return nil
}

func (s Store) WithDemandLock(id string, fn func() error) error {
	paths, err := s.prepareDemandPaths(id, false)
	if err != nil {
		return err
	}

	exists, err := demandWorkspaceExists(paths.demandDir)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("demand %s does not exist", id)
	}

	lockPath := filepath.Join(paths.demandDir, demandLockFile)
	return withFileLock(lockPath, 10*time.Second, fn)
}

func combineDemandLockResult(fnErr, cleanupErr error) error {
	if fnErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("%w (and %v)", fnErr, cleanupErr)
		}
		return fnErr
	}
	return cleanupErr
}

func joinDemandLockCleanupErrors(errs ...error) error {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func wrapDemandLockUnlockError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("unlock demand lock: %w", err)
}

func wrapDemandLockCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close demand lock: %w", err)
}

func (s Store) EnsureConfirmationEvidence(id, artifactName, confirmationID, record string, event Event) error {
	paths, err := s.prepareDemandPaths(id, false)
	if err != nil {
		return err
	}
	if err := validateAppendableArtifactName(artifactName); err != nil {
		return err
	}

	eventsPath := filepath.Join(paths.demandDir, EventsFile)
	events, err := readEventsLogRecoverTrailing(eventsPath)
	if err != nil {
		return err
	}
	eventExists := false
	if event.Data == nil {
		event.Data = map[string]string{}
	}
	for _, existing := range events {
		if existing.Data["confirmation_id"] == confirmationID {
			eventExists = true
			if existingRecord := existing.Data["record"]; existingRecord != "" {
				record = existingRecord
			}
			break
		}
	}

	startMarker := confirmationStartMarker(confirmationID)
	block := confirmationBlock(startMarker, confirmationEndMarker(confirmationID), record)
	artifactPath := filepath.Join(paths.demandDir, artifactName)
	artifactBody, err := os.ReadFile(artifactPath)
	if err != nil {
		return fmt.Errorf("read artifact %s: %w", artifactName, err)
	}
	artifactText := string(artifactBody)
	startMarkerCount := countMarkerLines(artifactText, startMarker)
	switch {
	case startMarkerCount == 1 && strings.Contains(artifactText, block):
	case startMarkerCount == 0:
		if err := s.AppendToArtifact(id, artifactName, confirmationAppendContent(artifactText, block)); err != nil {
			return err
		}
	default:
		repaired := repairConfirmationBlocks(artifactText, confirmationID, record)
		if err := writeTextAtomic(artifactPath, repaired); err != nil {
			return fmt.Errorf("rewrite artifact %s: %w", artifactName, err)
		}
	}

	if eventExists {
		return nil
	}

	event.Data["confirmation_id"] = confirmationID
	event.Data["record"] = record
	if err := s.AppendEvent(id, event); err != nil {
		return err
	}

	return nil
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

func writeTextAtomic(path, contents string) (err error) {
	tempFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp text file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err = tempFile.WriteString(contents); err != nil {
		tempFile.Close()
		return fmt.Errorf("write text file: %w", err)
	}
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("sync text file: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("close text file: %w", err)
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename text file: %w", err)
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

func validateAppendableArtifactName(name string) error {
	switch name {
	case IntakeFile,
		ContextFile,
		CodemapFile,
		PlanContextFile,
		ChangeScopeFile,
		RequirementsFile,
		PlanFile,
		ProgressFile,
		VerificationFile,
		CloseoutFile,
		MemoryCandidatesFile:
		return nil
	default:
		return fmt.Errorf("unsupported artifact %q", name)
	}
}

func confirmationStartMarker(id string) string {
	return fmt.Sprintf("<!-- devflow-confirmation:%s:start -->", id)
}

func confirmationEndMarker(id string) string {
	return fmt.Sprintf("<!-- devflow-confirmation:%s:end -->", id)
}

func confirmationBlock(startMarker, endMarker, record string) string {
	return startMarker + "\n" + strings.TrimSpace(record) + "\n" + endMarker + "\n"
}

func confirmationAppendContent(artifactText, block string) string {
	if artifactText == "" || strings.HasSuffix(artifactText, "\n\n") {
		return block
	}
	if strings.HasSuffix(artifactText, "\n") {
		return "\n" + block
	}
	return "\n\n" + block
}

func countMarkerLines(artifactText, marker string) int {
	count := 0
	for _, line := range strings.Split(artifactText, "\n") {
		if strings.TrimSpace(strings.TrimSuffix(line, "\r")) == marker {
			count++
		}
	}
	return count
}

func repairConfirmationBlocks(artifactText, confirmationID, record string) string {
	startMarker := confirmationStartMarker(confirmationID)
	endMarker := confirmationEndMarker(confirmationID)
	lines := strings.Split(artifactText, "\n")
	repaired := make([]string, 0, len(lines)+3)
	inserted := false

	for index := 0; index < len(lines); index++ {
		line := strings.TrimSuffix(lines[index], "\r")
		if strings.TrimSpace(line) != startMarker {
			repaired = append(repaired, lines[index])
			continue
		}

		endIndex := -1
		markerOnly := true
		for scan := index + 1; scan < len(lines); scan++ {
			next := strings.TrimSpace(strings.TrimSuffix(lines[scan], "\r"))
			if next == startMarker {
				break
			}
			if next == endMarker {
				endIndex = scan
				markerOnly = false
				break
			}
		}

		if !inserted {
			repaired = append(repaired, startMarker, strings.TrimSpace(record), endMarker)
			inserted = true
		}

		if markerOnly {
			continue
		}
		index = endIndex
	}

	if !inserted {
		return confirmationAppendContent(artifactText, confirmationBlock(startMarker, endMarker, record))
	}
	return strings.Join(repaired, "\n")
}

func readEventsLogRecoverTrailing(path string) ([]Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open events log: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	lastNonEmpty := -1
	for index, line := range lines {
		if strings.TrimSpace(line) != "" {
			lastNonEmpty = index
		}
	}

	var events []Event
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			lineNumber := index + 1
			if index != lastNonEmpty {
				return nil, fmt.Errorf("decode events log %s line %d: %w", filepath.Base(path), lineNumber, err)
			}

			validLog, encodeErr := encodeEventsLog(events)
			if encodeErr != nil {
				return nil, encodeErr
			}
			if writeErr := writeTextAtomic(path, validLog); writeErr != nil {
				return nil, fmt.Errorf("repair events log %s: %w", filepath.Base(path), writeErr)
			}

			badLineHash := sha256.Sum256([]byte(line))
			repairEvent := Event{
				Time:    time.Now().UTC(),
				Type:    "events.repaired",
				Message: "malformed trailing event removed",
				Data: map[string]string{
					"sha256": fmt.Sprintf("%x", badLineHash),
					"line":   strconv.Itoa(lineNumber),
				},
			}
			if appendErr := appendEventFile(path, repairEvent); appendErr != nil {
				return nil, fmt.Errorf("append events repair event: %w", appendErr)
			}
			events = append(events, repairEvent)
			return events, nil
		}
		events = append(events, event)
	}

	return events, nil
}

func encodeEventsLog(events []Event) (string, error) {
	var encoded strings.Builder
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return "", fmt.Errorf("encode events log: %w", err)
		}
		encoded.Write(data)
		encoded.WriteByte('\n')
	}
	return encoded.String(), nil
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
