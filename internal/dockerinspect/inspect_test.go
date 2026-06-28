package dockerinspect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileParsesDockerInspectArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inspect.json")
	payload := `[
  {
    "Id": "abc123",
    "Name": "/prowlarr",
    "Image": "sha256:runtime",
    "Config": {
      "Image": "lscr.io/linuxserver/prowlarr",
      "Labels": {"amud.enable": "true"},
      "Env": ["TZ=Europe/Prague", "PUID=99"]
    },
    "State": {"Status": "running"},
    "HostConfig": {"NetworkMode": "bridge"},
    "NetworkSettings": {
      "Ports": {
        "9696/tcp": [{"HostIp": "0.0.0.0", "HostPort": "9696"}]
      }
    }
  }
]`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	containers, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].Name != "prowlarr" {
		t.Fatalf("unexpected name: %s", containers[0].Name)
	}
	if containers[0].Ports[0].HostPort != "9696" {
		t.Fatalf("unexpected host port: %s", containers[0].Ports[0].HostPort)
	}
	if containers[0].Env["TZ"] != "Europe/Prague" {
		t.Fatalf("unexpected TZ: %s", containers[0].Env["TZ"])
	}
}
