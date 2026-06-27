// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package planfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const PlansDir = ".mewcode/plans"

var (
	planMu    sync.Mutex
	planPaths = map[string]string{}
)

func plansDir(workDir string) string {
	return filepath.Join(workDir, PlansDir)
}

func workDirKey(workDir string) string {
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return filepath.Clean(workDir)
	}
	return abs
}

func generateSlug() string {
	adjectives := []string{
		"bright", "calm", "bold", "swift", "quiet",
		"vivid", "clear", "keen", "warm", "cool",
		"sharp", "light", "deep", "pure", "soft",
	}
	nouns := []string{
		"plan", "draft", "design", "sketch", "blueprint",
		"outline", "strategy", "approach", "scheme", "map",
		"vision", "path", "route", "guide", "frame",
	}
	now := time.Now()
	ai := int(now.UnixNano()/1000) % len(adjectives)
	ni := int(now.UnixNano()/100) % len(nouns)
	return fmt.Sprintf("%s-%s-%s", adjectives[ai], nouns[ni], now.Format("0102-1504"))
}

func GetOrCreatePlanPath(workDir string) string {
	planMu.Lock()
	defer planMu.Unlock()

	key := workDirKey(workDir)
	if path := planPaths[key]; path != "" {
		return path
	}
	if path := findExistingPlanPath(workDir); path != "" {
		planPaths[key] = path
		return path
	}
	dir := plansDir(workDir)
	os.MkdirAll(dir, 0o755)
	slug := generateSlug()
	path := filepath.Join(dir, slug+".md")
	planPaths[key] = path
	return path
}

func GetPlanFilePath(workDir string) string {
	return GetOrCreatePlanPath(workDir)
}

func ResetPlanPath() {
	planMu.Lock()
	defer planMu.Unlock()
	planPaths = map[string]string{}
}

func PlanExists(workDir string) bool {
	return existingPlanPath(workDir) != ""
}

func LoadPlan(workDir string) (string, error) {
	path := existingPlanPath(workDir)
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func SavePlan(workDir, content string) error {
	path := GetOrCreatePlanPath(workDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func existingPlanPath(workDir string) string {
	planMu.Lock()
	defer planMu.Unlock()

	key := workDirKey(workDir)
	if path := planPaths[key]; path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		delete(planPaths, key)
	}
	path := findExistingPlanPath(workDir)
	if path != "" {
		planPaths[key] = path
	}
	return path
}

func findExistingPlanPath(workDir string) string {
	entries, err := os.ReadDir(plansDir(workDir))
	if err != nil {
		return ""
	}

	var selected string
	var selectedMod time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(plansDir(workDir), entry.Name())
		if selected == "" || info.ModTime().After(selectedMod) || (info.ModTime().Equal(selectedMod) && path > selected) {
			selected = path
			selectedMod = info.ModTime()
		}
	}
	return selected
}
