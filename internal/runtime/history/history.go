package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const maxEntries = 200

type entry struct {
	Text string `json:"text"`
	Ts   int64  `json:"ts"`
}

func devflowHistoryFilePath(dir string) string {
	return filepath.Join(dir, ".devflow", "prompt_history.jsonl")
}

func legacyHistoryFilePath(dir string) string {
	return filepath.Join(dir, ".mewcode", "prompt_history.jsonl")
}

// loadFrom reads the JSONL history file at path, returning the recorded
// prompt texts in order. Missing or unreadable files yield nil.
func loadFrom(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var texts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e entry
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.Text != "" {
			texts = append(texts, e.Text)
		}
	}
	return texts
}

// Load returns prompt history for a working directory, preferring the
// Devflow-owned .devflow file and falling back to the legacy .mewcode
// file only when the .devflow file does not exist.
func Load(dir string) []string {
	devflow := devflowHistoryFilePath(dir)
	if _, err := os.Stat(devflow); err == nil {
		return loadFrom(devflow)
	}
	return loadFrom(legacyHistoryFilePath(dir))
}

// Append records text to the Devflow-owned history file, suppressing an
// exact duplicate of the most recent entry and trimming to maxEntries.
func Append(dir string, text string) {
	path := devflowHistoryFilePath(dir)
	os.MkdirAll(filepath.Dir(path), 0o755)

	existing := Load(dir)

	if len(existing) > 0 && existing[len(existing)-1] == text {
		return
	}

	existing = append(existing, text)
	if len(existing) > maxEntries {
		existing = existing[len(existing)-maxEntries:]
	}

	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, t := range existing {
		data, _ := json.Marshal(entry{Text: t, Ts: time.Now().Unix()})
		w.Write(data)
		w.WriteByte('\n')
	}
	w.Flush()
}
