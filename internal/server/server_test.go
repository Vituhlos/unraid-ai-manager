package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"unraid-ai-manager/internal/approval"
)

func TestServerPlanAndApplyAMUD(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	backupDir := filepath.Join(dir, "backups")
	auditDir := filepath.Join(dir, "audit")
	plansDir := filepath.Join(dir, "plans")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templatesDir, "my-demo.xml")
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}

	helper, err := New(Config{
		TemplatesDir: templatesDir,
		BackupDir:    backupDir,
		AuditDir:     auditDir,
		PlansDir:     plansDir,
		LocalHost:    "192.0.2.10",
		APIKey:       "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(helper.Handler())
	defer httpServer.Close()

	unauthorized, err := http.Get(httpServer.URL + "/v1/inventory")
	if err != nil {
		t.Fatal(err)
	}
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthorized.StatusCode)
	}
	_ = unauthorized.Body.Close()

	planBody := `{"include_diffs":true,"save_plan":true}`
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/v1/plan/amud", strings.NewReader(planBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var planResponse AMUDPlanResponse
	if err := json.NewDecoder(response.Body).Decode(&planResponse); err != nil {
		t.Fatal(err)
	}
	if planResponse.Plan.PlanHash == "" {
		t.Fatal("missing plan hash")
	}
	if planResponse.PlanPath == "" {
		t.Fatal("missing saved plan path")
	}
	if len(planResponse.Diffs) != 1 || !planResponse.Diffs[0].Changed {
		t.Fatalf("expected one changed diff, got %#v", planResponse.Diffs)
	}

	applyPayload, err := json.Marshal(ApplyAMUDRequest{
		PlanPath:        planResponse.PlanPath,
		ConfirmPlanHash: planResponse.Plan.PlanHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	applyRequest, err := http.NewRequest(http.MethodPost, httpServer.URL+"/v1/apply/amud", bytes.NewReader(applyPayload))
	if err != nil {
		t.Fatal(err)
	}
	applyRequest.Header.Set("X-Unraid-AI-Key", "secret")
	applyRequest.Header.Set("Content-Type", "application/json")
	applyResponse, err := http.DefaultClient.Do(applyRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer applyResponse.Body.Close()
	if applyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 apply, got %d", applyResponse.StatusCode)
	}
	modified, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modified), `Target="amud.url"`) {
		t.Fatal("AMUD URL was not applied")
	}
}

func TestServerCapabilities(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	helper, err := New(Config{
		TemplatesDir: templatesDir,
		BackupDir:    filepath.Join(dir, "backups"),
		AuditDir:     filepath.Join(dir, "audit"),
		APIKey:       "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(helper.Handler())
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/v1/capabilities", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var capabilities CapabilitiesResponse
	if err := json.NewDecoder(response.Body).Decode(&capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.ToolSafety == "" {
		t.Fatal("missing tool safety summary")
	}
	foundDashboard := false
	foundDashboardSync := false
	foundDiscovery := false
	foundIntegrations := false
	foundCommunityApps := false
	for _, capability := range capabilities.Capabilities {
		if capability.ID == "dashboard-config" {
			foundDashboard = true
			if len(capability.Providers) != 1 || capability.Providers[0] != "amud" {
				t.Fatalf("unexpected dashboard providers: %#v", capability.Providers)
			}
		}
		if capability.ID == "dashboard-sync" {
			foundDashboardSync = true
		}
		if capability.ID == "integration-discovery" {
			foundDiscovery = true
		}
		if capability.ID == "dashboard-integrations" {
			foundIntegrations = true
		}
		if capability.ID == "community-applications" && capability.Status == "planned" {
			foundCommunityApps = true
		}
	}
	if !foundDashboard || !foundDashboardSync || !foundDiscovery || !foundIntegrations || !foundCommunityApps {
		t.Fatalf("missing expected capabilities: %#v", capabilities.Capabilities)
	}
}

func TestServerPlanAndApplyDashboardAMUDAdapter(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	backupDir := filepath.Join(dir, "backups")
	auditDir := filepath.Join(dir, "audit")
	plansDir := filepath.Join(dir, "plans")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templatesDir, "my-demo.xml")
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}

	helper, err := New(Config{
		TemplatesDir: templatesDir,
		BackupDir:    backupDir,
		AuditDir:     auditDir,
		PlansDir:     plansDir,
		LocalHost:    "192.0.2.10",
		APIKey:       "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(helper.Handler())
	defer httpServer.Close()

	planBody := `{"provider":"amud","include_diffs":true,"save_plan":true}`
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/v1/plan/dashboard", strings.NewReader(planBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var planResponse DashboardPlanResponse
	if err := json.NewDecoder(response.Body).Decode(&planResponse); err != nil {
		t.Fatal(err)
	}
	if planResponse.Plan.Kind != "dashboard-config" {
		t.Fatalf("unexpected dashboard plan kind: %s", planResponse.Plan.Kind)
	}
	if planResponse.Plan.Provider != "amud" || planResponse.Plan.Adapter != "dockerman-labels" {
		t.Fatalf("unexpected provider/adapter: %s/%s", planResponse.Plan.Provider, planResponse.Plan.Adapter)
	}
	if planResponse.PlanPath == "" {
		t.Fatal("missing saved dashboard plan path")
	}
	if len(planResponse.Diffs) != 1 || !planResponse.Diffs[0].Changed {
		t.Fatalf("expected one changed diff, got %#v", planResponse.Diffs)
	}

	applyPayload, err := json.Marshal(ApplyDashboardRequest{
		PlanPath:        planResponse.PlanPath,
		ConfirmPlanHash: planResponse.Plan.PlanHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	applyRequest, err := http.NewRequest(http.MethodPost, httpServer.URL+"/v1/apply/dashboard", bytes.NewReader(applyPayload))
	if err != nil {
		t.Fatal(err)
	}
	applyRequest.Header.Set("X-Unraid-AI-Key", "secret")
	applyRequest.Header.Set("Content-Type", "application/json")
	applyResponse, err := http.DefaultClient.Do(applyRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer applyResponse.Body.Close()
	if applyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 apply, got %d", applyResponse.StatusCode)
	}
	modified, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modified), `Target="amud.url"`) {
		t.Fatal("dashboard AMUD adapter did not apply AMUD URL")
	}
}

func TestServerPlanDashboardSync(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	plansDir := filepath.Join(dir, "plans")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templatesDir, "my-demo.xml")
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}
	inspectPath := filepath.Join(dir, "inspect.json")
	inspectJSON := `[{
  "Id": "demo-id",
  "Name": "/Demo",
  "Image": "demo/demo",
  "Config": {"Image": "demo/demo", "Labels": {}, "Env": []},
  "State": {"Status": "running"},
  "HostConfig": {"NetworkMode": "bridge", "PortBindings": {"8080/tcp": [{"HostIp": "0.0.0.0", "HostPort": "18080"}]}},
  "NetworkSettings": {"Ports": {"8080/tcp": [{"HostIp": "0.0.0.0", "HostPort": "18080"}]}}
}]`
	if err := os.WriteFile(inspectPath, []byte(inspectJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	helper, err := New(Config{
		TemplatesDir: templatesDir,
		BackupDir:    filepath.Join(dir, "backups"),
		AuditDir:     filepath.Join(dir, "audit"),
		PlansDir:     plansDir,
		LocalHost:    "192.0.2.10",
		APIKey:       "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(helper.Handler())
	defer httpServer.Close()

	planBody := `{"provider":"amud","include_diffs":true,"save_plan":true,"inspect_path":` + strconv.Quote(inspectPath) + `}`
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/v1/plan/dashboard-sync", strings.NewReader(planBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var planResponse DashboardSyncPlanResponse
	if err := json.NewDecoder(response.Body).Decode(&planResponse); err != nil {
		t.Fatal(err)
	}
	if planResponse.Plan.Kind != "dashboard-sync" {
		t.Fatalf("unexpected plan kind: %s", planResponse.Plan.Kind)
	}
	if planResponse.Plan.PlanHash == "" || planResponse.Plan.DashboardPlan.PlanHash == "" || planResponse.Plan.RecreatePlan.PlanHash == "" {
		t.Fatalf("missing plan hashes: %#v", planResponse.Plan)
	}
	if len(planResponse.Plan.RecreatePlan.Entries) != 1 {
		t.Fatalf("expected one recreate entry, got %#v", planResponse.Plan.RecreatePlan.Entries)
	}
	if planResponse.PlanPath == "" {
		t.Fatal("missing saved sync plan path")
	}
	if len(planResponse.Diffs) != 1 || !planResponse.Diffs[0].Changed {
		t.Fatalf("expected one changed diff, got %#v", planResponse.Diffs)
	}
}

func TestServerPlanDashboardIntegrations(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	configDir := filepath.Join(dir, "radarr-config")
	plansDir := filepath.Join(dir, "plans")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.xml"), []byte(`<Config><ApiKey>radarr-secret-value</ApiKey></Config>`), 0o600); err != nil {
		t.Fatal(err)
	}
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>radarr</Name>
  <Repository>lscr.io/linuxserver/radarr:latest</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:7878]/</WebUI>
  <Config Name="WebUI" Target="7878" Default="7878" Mode="tcp" Type="Port">7878</Config>
  <Config Name="Config" Target="/config" Default="" Mode="rw" Type="Path">` + configDir + `</Config>
</Container>
`
	if err := os.WriteFile(filepath.Join(templatesDir, "my-radarr.xml"), []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}
	helper, err := New(Config{
		TemplatesDir: templatesDir,
		BackupDir:    filepath.Join(dir, "backups"),
		AuditDir:     filepath.Join(dir, "audit"),
		PlansDir:     plansDir,
		LocalHost:    "192.0.2.10",
		APIKey:       "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(helper.Handler())
	defer httpServer.Close()

	planBody := `{"provider":"amud","runtime_filter":"templates","save_plan":true}`
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/v1/plan/dashboard-integrations", strings.NewReader(planBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var planResponse DashboardIntegrationPlanResponse
	if err := json.NewDecoder(response.Body).Decode(&planResponse); err != nil {
		t.Fatal(err)
	}
	if planResponse.Plan.Kind != "dashboard-integrations" || planResponse.Plan.PlanHash == "" {
		t.Fatalf("unexpected integration plan: %#v", planResponse.Plan)
	}
	if len(planResponse.Plan.Entries) != 1 {
		t.Fatalf("expected one integration entry, got %#v", planResponse.Plan.Entries)
	}
	entry := planResponse.Plan.Entries[0]
	if entry.Status != "ready" {
		t.Fatalf("expected ready entry, got %#v", entry)
	}
	if len(entry.RequiredSecrets) != 1 || entry.RequiredSecrets[0].Ref == "" {
		t.Fatalf("expected required secret ref, got %#v", entry.RequiredSecrets)
	}
	if entry.RequiredSecrets[0].Preview == "radarr-secret-value" {
		t.Fatal("integration plan leaked full secret")
	}
	if planResponse.PlanPath == "" {
		t.Fatal("missing saved integration plan path")
	}
}

func TestServerApplyRequiresApprovalToken(t *testing.T) {
	dir := t.TempDir()
	templatesDir := filepath.Join(dir, "templates")
	backupDir := filepath.Join(dir, "backups")
	auditDir := filepath.Join(dir, "audit")
	plansDir := filepath.Join(dir, "plans")
	approvalsDir := filepath.Join(dir, "approvals")
	if err := os.MkdirAll(templatesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join(templatesDir, "my-demo.xml")
	templateXML := `<?xml version="1.0"?>
<Container version="2">
  <Name>Demo</Name>
  <Repository>demo/demo</Repository>
  <Network>bridge</Network>
  <Privileged>false</Privileged>
  <WebUI>http://[IP]:[PORT:8080]/</WebUI>
  <Config Name="WebUI" Target="8080" Default="8080" Mode="tcp" Type="Port">18080</Config>
  <TailscaleStateDir/>
</Container>
`
	if err := os.WriteFile(templatePath, []byte(templateXML), 0o600); err != nil {
		t.Fatal(err)
	}

	helper, err := New(Config{
		TemplatesDir:         templatesDir,
		BackupDir:            backupDir,
		AuditDir:             auditDir,
		PlansDir:             plansDir,
		ApprovalsDir:         approvalsDir,
		LocalHost:            "192.0.2.10",
		APIKey:               "secret",
		RequireApprovalToken: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(helper.Handler())
	defer httpServer.Close()

	planResponse := createAMUDPlanForTest(t, httpServer.URL)

	applyPayload, err := json.Marshal(ApplyAMUDRequest{
		PlanPath:        planResponse.PlanPath,
		ConfirmPlanHash: planResponse.Plan.PlanHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	status := postForStatus(t, httpServer.URL+"/v1/apply/amud", applyPayload)
	if status != http.StatusBadRequest {
		t.Fatalf("expected apply without approval to fail with 400, got %d", status)
	}

	grant, err := approval.Grant(approvalsDir, planResponse.Plan.PlanHash, "amud", time.Minute, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	applyPayload, err = json.Marshal(ApplyAMUDRequest{
		PlanPath:        planResponse.PlanPath,
		ConfirmPlanHash: planResponse.Plan.PlanHash,
		ApprovalToken:   grant.Token,
	})
	if err != nil {
		t.Fatal(err)
	}
	status = postForStatus(t, httpServer.URL+"/v1/apply/amud", applyPayload)
	if status != http.StatusOK {
		t.Fatalf("expected approved apply to succeed with 200, got %d", status)
	}
}

func createAMUDPlanForTest(t *testing.T, baseURL string) AMUDPlanResponse {
	t.Helper()
	planBody := `{"include_diffs":true,"save_plan":true}`
	request, err := http.NewRequest(http.MethodPost, baseURL+"/v1/plan/amud", strings.NewReader(planBody))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 plan, got %d", response.StatusCode)
	}
	var planResponse AMUDPlanResponse
	if err := json.NewDecoder(response.Body).Decode(&planResponse); err != nil {
		t.Fatal(err)
	}
	return planResponse
}

func postForStatus(t *testing.T, url string, payload []byte) int {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Unraid-AI-Key", "secret")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	return response.StatusCode
}
