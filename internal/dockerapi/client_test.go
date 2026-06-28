package dockerapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInspectAllViaHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/containers/json":
			_, _ = writer.Write([]byte(`[{"Id":"abc123","Names":["/prowlarr"],"Image":"lscr.io/linuxserver/prowlarr","State":"running","Status":"Up"}]`))
		case "/containers/abc123/json":
			_, _ = writer.Write([]byte(`{
  "Id": "abc123",
  "Name": "/prowlarr",
  "Config": {
    "Image": "lscr.io/linuxserver/prowlarr",
    "Labels": {"amud.enable": "true"},
    "Env": ["TZ=Europe/Prague"]
  },
  "State": {"Status": "running"},
  "HostConfig": {"NetworkMode": "bridge"},
  "NetworkSettings": {"Ports": {"9696/tcp": [{"HostIp": "0.0.0.0", "HostPort": "9696"}]}}
}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	containers, err := client.InspectAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}
	if containers[0].Name != "prowlarr" {
		t.Fatalf("unexpected name: %s", containers[0].Name)
	}
	if containers[0].Labels["amud.enable"] != "true" {
		t.Fatalf("unexpected labels: %#v", containers[0].Labels)
	}
}
