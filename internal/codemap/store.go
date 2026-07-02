package codemap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirName     = ".devflow/codemap"
	IndexFile   = "index.json"
	SummaryFile = "summary.md"
)

func WriteIndex(root string, index Index) error {
	dir := filepath.Join(root, filepath.FromSlash(DirName))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create codemap dir: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codemap index: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, IndexFile), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write codemap index: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SummaryFile), []byte(RenderSummary(index)), 0o644); err != nil {
		return fmt.Errorf("write codemap summary: %w", err)
	}
	return nil
}

func ReadIndex(root string) (Index, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(DirName), IndexFile))
	if err != nil {
		return Index{}, fmt.Errorf("read codemap index: %w", err)
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return Index{}, fmt.Errorf("decode codemap index: %w", err)
	}
	return index, nil
}
