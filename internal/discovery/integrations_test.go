package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerxml"
)

func TestDiscoverIntegrationsFindsArrAPIKeyWithoutRevealingIt(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "radarr")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.xml"), []byte(`<Config><ApiKey>abcdef1234567890</ApiKey></Config>`), 0o600); err != nil {
		t.Fatal(err)
	}
	template := dockerxml.Template{
		Name:       "radarr",
		Repository: "lscr.io/linuxserver/radarr:latest",
		Configs: []dockerxml.ConfigEntry{
			{Type: "Path", Target: "/config", Value: configDir},
		},
	}
	report := DiscoverIntegrations([]dockerxml.Template{template}, Options{})
	if len(report.Records) != 1 {
		t.Fatalf("expected one record, got %#v", report.Records)
	}
	record := report.Records[0]
	if record.ServiceType != "radarr" {
		t.Fatalf("expected radarr service, got %s", record.ServiceType)
	}
	if len(record.Secrets) != 1 || !record.Secrets[0].Found {
		t.Fatalf("expected discovered secret, got %#v", record.Secrets)
	}
	if record.Secrets[0].Preview == "abcdef1234567890" {
		t.Fatal("secret preview leaked the full value")
	}
	if record.Secrets[0].Length != 16 {
		t.Fatalf("expected original secret length, got %d", record.Secrets[0].Length)
	}
}
