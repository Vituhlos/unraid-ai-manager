package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
