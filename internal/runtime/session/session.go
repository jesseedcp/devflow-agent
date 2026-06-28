package session

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Ts      int64  `json:"ts"`
}

type SessionInfo struct {
	ID           string
	FirstMessage string
	MessageCount int
	FileSize     int64
	GitBranch    string
	ModTime      time.Time
}

func NewID() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 极少失败；兜底用纳秒低 16 位，仍能避免同秒同进程冲突
		return fmt.Sprintf("%s-%04x", time.Now().Format("20060102-150405"), time.Now().UnixNano()&0xFFFF)
	}
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

// sessionsDir is the Devflow-owned session directory. All new writes go here.
func sessionsDir(workDir string) string {
	return filepath.Join(workDir, ".devflow", "sessions")
}

// legacySessionsDir is the read-only MewCode session directory used as a
// fallback when no same-ID Devflow session exists.
func legacySessionsDir(workDir string) string {
	return filepath.Join(workDir, ".mewcode", "sessions")
}

func sessionFilePath(workDir, id string) string {
	return filepath.Join(sessionsDir(workDir), id+".jsonl")
}

func legacySessionFilePath(workDir, id string) string {
	return filepath.Join(legacySessionsDir(workDir), id+".jsonl")
}

func SaveMessage(workDir, sessionID string, msg Message) {
	dir := sessionsDir(workDir)
	os.MkdirAll(dir, 0o755)

	f, err := os.OpenFile(sessionFilePath(workDir, sessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	data, _ := json.Marshal(msg)
	f.Write(data)
	f.Write([]byte("\n"))
}

// loadFrom reads the JSONL messages recorded at path, returning parsed
// messages in order. Missing or unreadable files yield nil.
func loadFrom(path string) []Message {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		var msg Message
		if json.Unmarshal(scanner.Bytes(), &msg) == nil && msg.Content != "" {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

// LoadSession returns the messages for sessionID, preferring the
// Devflow-owned session file and falling back to the legacy MewCode file
// only when the Devflow file does not exist.
func LoadSession(workDir, sessionID string) []Message {
	if msgs := loadFrom(sessionFilePath(workDir, sessionID)); msgs != nil {
		return msgs
	}
	return loadFrom(legacySessionFilePath(workDir, sessionID))
}

type dirEntryInfo struct {
	id       string
	fileSize int64
	modTime  time.Time
	legacy   bool
}

func readSessionEntries(dir string, legacy bool) []dirEntryInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var infos []dirEntryInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, dirEntryInfo{
			id:       strings.TrimSuffix(e.Name(), ".jsonl"),
			fileSize: info.Size(),
			modTime:  info.ModTime(),
			legacy:   legacy,
		})
	}
	return infos
}

func ListSessions(workDir string) []SessionInfo {
	branch := currentGitBranch(workDir)

	devflow := readSessionEntries(sessionsDir(workDir), false)
	legacy := readSessionEntries(legacySessionsDir(workDir), true)

	seen := make(map[string]bool)
	var infos []dirEntryInfo
	for _, info := range devflow {
		seen[info.id] = true
		infos = append(infos, info)
	}
	for _, info := range legacy {
		if seen[info.id] {
			continue
		}
		infos = append(infos, info)
	}

	var sessions []SessionInfo
	for _, info := range infos {
		msgs := LoadSession(workDir, info.id)
		first := ""
		for _, msg := range msgs {
			if msg.Role == "user" {
				first = msg.Content
				break
			}
		}

		sessions = append(sessions, SessionInfo{
			ID:           info.id,
			FirstMessage: first,
			MessageCount: len(msgs),
			FileSize:     info.fileSize,
			GitBranch:    branch,
			ModTime:      info.modTime,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	return sessions
}

func currentGitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func FormatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
}

func FormatFileSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%dB", bytes)
	case bytes < 1024*1024:
		kb := float64(bytes) / 1024
		if kb == float64(int(kb)) {
			return fmt.Sprintf("%.0fKB", kb)
		}
		return fmt.Sprintf("%.1fKB", kb)
	default:
		mb := float64(bytes) / 1024 / 1024
		return fmt.Sprintf("%.1fMB", mb)
	}
}

func MatchesSearch(s SessionInfo, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(s.FirstMessage), q) ||
		strings.Contains(strings.ToLower(s.ID), q)
}
