package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewID(t *testing.T) {
	id := NewID()
	if len(id) != 20 { // 20060102-150405-xxxx
		t.Fatalf("unexpected ID format: %s (len=%d)", id, len(id))
	}
	// 同秒生成两个 ID 不应相同
	id2 := NewID()
	if id == id2 {
		t.Fatalf("two IDs generated in same second collided: %s", id)
	}
}

func TestSaveMessageWritesToDevflow(t *testing.T) {
	dir := t.TempDir()
	sid := "test-session"

	SaveMessage(dir, sid, Message{Role: "user", Content: "hello", Ts: 1})
	SaveMessage(dir, sid, Message{Role: "assistant", Content: "hi", Ts: 2})

	msgs := LoadSession(dir, sid)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Fatalf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi" {
		t.Fatalf("unexpected second message: %+v", msgs[1])
	}

	path := filepath.Join(dir, ".devflow", "sessions", sid+".jsonl")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("devflow session file was not created")
	}

	legacy := filepath.Join(dir, ".mewcode", "sessions", sid+".jsonl")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("legacy mewcode session file should not be created")
	}
}

func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	msgs := LoadSession(dir, "nonexistent")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

func TestLoadSessionPrefersDevflow(t *testing.T) {
	dir := t.TempDir()
	sid := "dup-session"

	if err := os.MkdirAll(filepath.Join(dir, ".mewcode", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := legacySessionFilePath(dir, sid)
	if err := os.WriteFile(legacy, []byte(`{"role":"user","content":"legacy","ts":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, ".devflow", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	devflow := sessionFilePath(dir, sid)
	if err := os.WriteFile(devflow, []byte(`{"role":"user","content":"devflow","ts":2}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msgs := LoadSession(dir, sid)
	if len(msgs) != 1 || msgs[0].Content != "devflow" {
		t.Fatalf("expected devflow message, got %v", msgs)
	}
}

func TestLoadSessionFallsBackToMewcode(t *testing.T) {
	dir := t.TempDir()
	sid := "legacy-session"

	if err := os.MkdirAll(filepath.Join(dir, ".mewcode", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := legacySessionFilePath(dir, sid)
	if err := os.WriteFile(legacy, []byte(`{"role":"user","content":"legacy","ts":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msgs := LoadSession(dir, sid)
	if len(msgs) != 1 || msgs[0].Content != "legacy" {
		t.Fatalf("expected legacy message, got %v", msgs)
	}
}

func TestListSessionsNewestFirstAndDedup(t *testing.T) {
	dir := t.TempDir()

	SaveMessage(dir, "s1", Message{Role: "user", Content: "first session", Ts: 1})
	time.Sleep(20 * time.Millisecond)
	SaveMessage(dir, "s2", Message{Role: "user", Content: "second session", Ts: 2})
	SaveMessage(dir, "s2", Message{Role: "assistant", Content: "reply", Ts: 3})

	// Legacy-only session that should appear because there is no Devflow match.
	if err := os.MkdirAll(filepath.Join(dir, ".mewcode", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := legacySessionFilePath(dir, "legacy-only")
	if err := os.WriteFile(legacy, []byte(`{"role":"user","content":"legacy only","ts":4}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Legacy session sharing an ID with a Devflow session must be deduped away.
	legacyDup := legacySessionFilePath(dir, "s2")
	if err := os.WriteFile(legacyDup, []byte(`{"role":"user","content":"legacy dup","ts":4}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions := ListSessions(dir)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d: %+v", len(sessions), sessions)
	}

	if sessions[0].ModTime.Before(sessions[1].ModTime) {
		t.Fatalf("expected newest first, got %v then %v", sessions[0].ModTime, sessions[1].ModTime)
	}

	ids := make(map[string]bool)
	for _, s := range sessions {
		ids[s.ID] = true
		if s.ID == "s2" && s.MessageCount != 2 {
			t.Fatalf("expected 2 messages in s2, got %d", s.MessageCount)
		}
		if s.ID == "s2" && s.FirstMessage == "legacy dup" {
			t.Fatalf("legacy dup should have been deduped in favor of devflow")
		}
	}
	if !ids["s1"] || !ids["s2"] || !ids["legacy-only"] {
		t.Fatalf("missing expected sessions: %v", ids)
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()
	if got := FormatRelativeTime(now.Add(-30 * time.Second)); got != "just now" {
		t.Fatalf("expected 'just now', got %s", got)
	}
	if got := FormatRelativeTime(now.Add(-5 * time.Minute)); got != "5 minutes ago" {
		t.Fatalf("expected '5 minutes ago', got %s", got)
	}
	if got := FormatRelativeTime(now.Add(-3 * time.Hour)); got != "3 hours ago" {
		t.Fatalf("expected '3 hours ago', got %s", got)
	}
}

func TestFormatFileSize(t *testing.T) {
	if got := FormatFileSize(500); got != "500B" {
		t.Fatalf("expected '500B', got %s", got)
	}
	if got := FormatFileSize(53862); got != "52.6KB" {
		t.Fatalf("expected '52.6KB', got %s", got)
	}
}

func TestMatchesSearch(t *testing.T) {
	s := SessionInfo{FirstMessage: "Hello World", ID: "test-123"}
	if !MatchesSearch(s, "hello") {
		t.Fatal("should match case-insensitive")
	}
	if !MatchesSearch(s, "") {
		t.Fatal("empty query should match all")
	}
	if MatchesSearch(s, "zzz") {
		t.Fatal("should not match unrelated queries")
	}
}
