package approval

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGrantAndConsume(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	grant, err := Grant(dir, "abc123", "amud", time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Token == "" {
		t.Fatal("missing token")
	}
	if err := Consume(dir, "abc123", grant.Token, now.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := Consume(dir, "abc123", grant.Token, now.Add(20*time.Second)); err == nil {
		t.Fatal("expected used token error")
	}
}

func TestExtractPlanHashFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, []byte(`{"plan":{"plan_hash":"abc123"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, err := ExtractPlanHashFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash != "abc123" {
		t.Fatalf("unexpected hash: %s", hash)
	}
}
