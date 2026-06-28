package planner

import (
	"os"
	"path/filepath"
	"testing"

	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
)

func TestParseWebUIContainerPort(t *testing.T) {
	port := ParseWebUIContainerPort("http://[IP]:[PORT:9696]/system/status")
	if port == nil || *port != 9696 {
		t.Fatalf("expected 9696, got %#v", port)
	}
	if ParseWebUIContainerPort("") != nil {
		t.Fatal("expected nil port")
	}
}

func TestAMUDPlanLocalURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-demo.xml")
	xml := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <TemplateURL>https://example.test/demo.xml</TemplateURL>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
</Container>`
	if err := os.WriteFile(path, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plan := BuildAMUDPlan([]dockerxml.Template{template}, AMUDOptions{LocalHost: "192.0.2.10"})
	if got := plan.Entries[0].ProposedLabels["amud.url"]; got != "http://192.0.2.10:18080" {
		t.Fatalf("unexpected amud.url: %s", got)
	}
	if got := plan.Entries[0].ProposedLabels["amud.icon"]; got != "demo" {
		t.Fatalf("unexpected amud.icon: %s", got)
	}
}

func TestAMUDPlanSkipsPortOnlyTemplatesByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.xml")
	xml := `<?xml version="1.0"?>
<Container version="2">
  <Name>mariadb</Name>
  <Repository>mariadb:latest</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <Config Name="DB Port" Target="3306" Default="3306" Mode="tcp" Type="Port">3306</Config>
</Container>`
	if err := os.WriteFile(path, []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	template, err := dockerxml.ParseTemplateFile(path)
	if err != nil {
		t.Fatal(err)
	}

	plan := BuildAMUDPlan([]dockerxml.Template{template}, AMUDOptions{LocalHost: "192.0.2.10"})
	if len(plan.Entries) != 0 {
		t.Fatalf("expected port-only template to be skipped by default, got %#v", plan.Entries)
	}

	plan = BuildAMUDPlan([]dockerxml.Template{template}, AMUDOptions{LocalHost: "192.0.2.10", IncludePortOnly: true})
	if len(plan.Entries) != 1 {
		t.Fatalf("expected port-only template when IncludePortOnly=true, got %#v", plan.Entries)
	}
	if got := plan.Entries[0].URL.URL; got != "http://192.0.2.10:3306" {
		t.Fatalf("unexpected port-only URL: %s", got)
	}
}

func TestAMUDPlanContainerFilters(t *testing.T) {
	dir := t.TempDir()
	webPath := filepath.Join(dir, "web.xml")
	webXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>WebApp</Name>
  <Repository>demo/web</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
</Container>`
	otherPath := filepath.Join(dir, "other.xml")
	otherXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>OtherApp</Name>
  <Repository>demo/other</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:9090]/</WebUI>
  <Config Name="WebUI" Target="9090" Default="9090" Mode="tcp" Type="Port">19090</Config>
</Container>`
	if err := os.WriteFile(webPath, []byte(webXML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte(otherXML), 0o600); err != nil {
		t.Fatal(err)
	}
	webTemplate, err := dockerxml.ParseTemplateFile(webPath)
	if err != nil {
		t.Fatal(err)
	}
	otherTemplate, err := dockerxml.ParseTemplateFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}

	plan := BuildAMUDPlan([]dockerxml.Template{webTemplate, otherTemplate}, AMUDOptions{
		LocalHost:     "192.0.2.10",
		Names:         map[string]bool{"webapp": true},
		ExcludedNames: map[string]bool{"otherapp": true},
	})
	if len(plan.Entries) != 1 || plan.Entries[0].Container != "WebApp" {
		t.Fatalf("expected only WebApp, got %#v", plan.Entries)
	}

	plan = BuildAMUDPlan([]dockerxml.Template{webTemplate, otherTemplate}, AMUDOptions{
		LocalHost:     "192.0.2.10",
		ExcludedNames: map[string]bool{"webapp": true},
	})
	if len(plan.Entries) != 1 || plan.Entries[0].Container != "OtherApp" {
		t.Fatalf("expected only OtherApp, got %#v", plan.Entries)
	}
}

func TestFilterTemplatesByRuntime(t *testing.T) {
	dir := t.TempDir()
	runningPath := filepath.Join(dir, "running.xml")
	runningXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>RunningApp</Name>
  <Repository>demo/running</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
</Container>`
	exitedPath := filepath.Join(dir, "exited.xml")
	exitedXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>ExitedApp</Name>
  <Repository>demo/exited</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:9090]/</WebUI>
  <Config Name="WebUI" Target="9090" Default="9090" Mode="tcp" Type="Port">19090</Config>
</Container>`
	missingPath := filepath.Join(dir, "missing.xml")
	missingXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>MissingApp</Name>
  <Repository>demo/missing</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:7070]/</WebUI>
  <Config Name="WebUI" Target="7070" Default="7070" Mode="tcp" Type="Port">17070</Config>
</Container>`
	for path, payload := range map[string]string{
		runningPath: runningXML,
		exitedPath:  exitedXML,
		missingPath: missingXML,
	} {
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runningTemplate, err := dockerxml.ParseTemplateFile(runningPath)
	if err != nil {
		t.Fatal(err)
	}
	exitedTemplate, err := dockerxml.ParseTemplateFile(exitedPath)
	if err != nil {
		t.Fatal(err)
	}
	missingTemplate, err := dockerxml.ParseTemplateFile(missingPath)
	if err != nil {
		t.Fatal(err)
	}
	templates := []dockerxml.Template{runningTemplate, exitedTemplate, missingTemplate}
	runtime := []dockerinspect.Container{
		{Name: "RunningApp", State: "running"},
		{Name: "ExitedApp", State: "exited"},
	}

	existing, err := FilterTemplatesByRuntime(templates, runtime, "existing")
	if err != nil {
		t.Fatal(err)
	}
	if len(existing) != 2 {
		t.Fatalf("expected 2 existing templates, got %#v", existing)
	}

	running, err := FilterTemplatesByRuntime(templates, runtime, "running")
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 1 || running[0].Name != "RunningApp" {
		t.Fatalf("expected only RunningApp, got %#v", running)
	}
}
