package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"unraid-ai-manager/internal/approval"
	"unraid-ai-manager/internal/compare"
	"unraid-ai-manager/internal/dockerapi"
	"unraid-ai-manager/internal/dockerinspect"
	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/executor"
	"unraid-ai-manager/internal/lifecycle"
	"unraid-ai-manager/internal/planner"
	"unraid-ai-manager/internal/risk"
	"unraid-ai-manager/internal/textdiff"
	"unraid-ai-manager/internal/xmlpatch"
)

type Config struct {
	ListenAddr           string
	TemplatesDir         string
	BackupDir            string
	AuditDir             string
	PlansDir             string
	ApprovalsDir         string
	DockerSocket         string
	DockerHost           string
	LocalHost            string
	APIKey               string
	RequireApprovalToken bool
	Logger               *slog.Logger
}

type Server struct {
	config Config
	mux    *http.ServeMux
	logger *slog.Logger
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type InventoryResponse struct {
	WriteEnabled  bool              `json:"write_enabled"`
	TemplateCount int               `json:"template_count"`
	Containers    []ContainerRecord `json:"containers"`
}

type ContainerRecord struct {
	SourcePath   string                  `json:"source_path"`
	Version      string                  `json:"version"`
	Name         string                  `json:"name"`
	Repository   string                  `json:"repository"`
	Registry     string                  `json:"registry"`
	Network      string                  `json:"network"`
	Privileged   bool                    `json:"privileged"`
	WebUI        string                  `json:"web_ui"`
	TemplateURL  string                  `json:"template_url"`
	Icon         string                  `json:"icon"`
	ExtraParams  string                  `json:"extra_params"`
	Ports        []dockerxml.ConfigEntry `json:"ports"`
	Paths        []dockerxml.ConfigEntry `json:"paths"`
	Variables    []dockerxml.ConfigEntry `json:"variables"`
	Labels       []dockerxml.ConfigEntry `json:"labels"`
	RiskFindings []risk.Finding          `json:"risk_findings"`
}

type CapabilitiesResponse struct {
	Version      string             `json:"version"`
	ToolSafety   string             `json:"tool_safety"`
	Capabilities []CapabilityRecord `json:"capabilities"`
}

type CapabilityRecord struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Access      string   `json:"access"`
	Description string   `json:"description"`
	Providers   []string `json:"providers,omitempty"`
	Actions     []string `json:"actions,omitempty"`
}

type DiffRecord struct {
	Container   string               `json:"container"`
	SourcePath  string               `json:"source_path"`
	Changed     bool                 `json:"changed"`
	Operations  []xmlpatch.Operation `json:"operations"`
	UnifiedDiff string               `json:"unified_diff"`
}

type AMUDPlanRequest struct {
	LocalHost         string            `json:"local_host"`
	URLMode           string            `json:"url_mode"`
	CloudflareDomain  string            `json:"cloudflare_domain"`
	CloudflareRoutes  map[string]string `json:"cloudflare_routes"`
	Containers        []string          `json:"containers"`
	ExcludeContainers []string          `json:"exclude_containers"`
	IncludePortOnly   bool              `json:"include_port_only"`
	RuntimeFilter     string            `json:"runtime_filter"`
	InspectPath       string            `json:"inspect_path"`
	IncludeDiffs      bool              `json:"include_diffs"`
	SavePlan          bool              `json:"save_plan"`
}

type AMUDPlanResponse struct {
	Plan     planner.AMUDPlan `json:"plan"`
	Diffs    []DiffRecord     `json:"diffs,omitempty"`
	PlanPath string           `json:"plan_path,omitempty"`
}

type DashboardPlanRequest struct {
	Provider          string            `json:"provider"`
	LocalHost         string            `json:"local_host"`
	URLMode           string            `json:"url_mode"`
	CloudflareDomain  string            `json:"cloudflare_domain"`
	CloudflareRoutes  map[string]string `json:"cloudflare_routes"`
	Containers        []string          `json:"containers"`
	ExcludeContainers []string          `json:"exclude_containers"`
	IncludePortOnly   bool              `json:"include_port_only"`
	RuntimeFilter     string            `json:"runtime_filter"`
	InspectPath       string            `json:"inspect_path"`
	IncludeDiffs      bool              `json:"include_diffs"`
	SavePlan          bool              `json:"save_plan"`
}

type DashboardPlanResponse struct {
	Plan     planner.DashboardPlan `json:"plan"`
	Diffs    []DiffRecord          `json:"diffs,omitempty"`
	PlanPath string                `json:"plan_path,omitempty"`
}

type TZPlanRequest struct {
	Timezone         string   `json:"timezone"`
	Containers       []string `json:"containers"`
	IncludeUnchanged bool     `json:"include_unchanged"`
	IncludeDiffs     bool     `json:"include_diffs"`
	SavePlan         bool     `json:"save_plan"`
}

type TZPlanResponse struct {
	Plan     planner.TZPlan `json:"plan"`
	Diffs    []DiffRecord   `json:"diffs,omitempty"`
	PlanPath string         `json:"plan_path,omitempty"`
}

type RecreatePlanRequest struct {
	InspectPath string   `json:"inspect_path"`
	Containers  []string `json:"containers"`
	All         bool     `json:"all"`
	SavePlan    bool     `json:"save_plan"`
}

type RecreatePlanResponse struct {
	Plan     lifecycle.RecreatePlan `json:"plan"`
	PlanPath string                 `json:"plan_path,omitempty"`
}

type ApplyAMUDRequest struct {
	PlanPath        string            `json:"plan_path,omitempty"`
	Plan            *planner.AMUDPlan `json:"plan,omitempty"`
	ConfirmPlanHash string            `json:"confirm_plan_hash"`
	ApprovalToken   string            `json:"approval_token,omitempty"`
}

type ApplyDashboardRequest struct {
	PlanPath        string                 `json:"plan_path,omitempty"`
	Plan            *planner.DashboardPlan `json:"plan,omitempty"`
	ConfirmPlanHash string                 `json:"confirm_plan_hash"`
	ApprovalToken   string                 `json:"approval_token,omitempty"`
}

type ApplyTZRequest struct {
	PlanPath        string          `json:"plan_path,omitempty"`
	Plan            *planner.TZPlan `json:"plan,omitempty"`
	ConfirmPlanHash string          `json:"confirm_plan_hash"`
	ApprovalToken   string          `json:"approval_token,omitempty"`
}

type ApplyRecreateRequest struct {
	PlanPath        string                  `json:"plan_path,omitempty"`
	Plan            *lifecycle.RecreatePlan `json:"plan,omitempty"`
	ConfirmPlanHash string                  `json:"confirm_plan_hash"`
	ApprovalToken   string                  `json:"approval_token,omitempty"`
}

type RestoreXMLRequest struct {
	BackupPath          string `json:"backup_path"`
	TargetPath          string `json:"target_path"`
	ConfirmBackupSHA256 string `json:"confirm_backup_sha256"`
}

func New(config Config) (*Server, error) {
	if config.ListenAddr == "" {
		config.ListenAddr = "127.0.0.1:37231"
	}
	if config.TemplatesDir == "" {
		return nil, errors.New("templates dir is required")
	}
	if config.BackupDir == "" {
		return nil, errors.New("backup dir is required")
	}
	if config.AuditDir == "" {
		return nil, errors.New("audit dir is required")
	}
	if config.PlansDir == "" {
		config.PlansDir = filepath.Join(config.AuditDir, "plans")
	}
	if config.ApprovalsDir == "" {
		config.ApprovalsDir = filepath.Join(config.AuditDir, "approvals")
	}
	if config.LocalHost == "" {
		config.LocalHost = "127.0.0.1"
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	server := &Server{
		config: config,
		mux:    http.NewServeMux(),
		logger: logger,
	}
	server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.withLogging(s.withAuth(s.mux))
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:              s.config.ListenAddr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	s.logger.Info("unraid helper listening", "addr", listener.Addr().String())
	err = httpServer.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/capabilities", s.handleCapabilities)
	s.mux.HandleFunc("GET /v1/inventory", s.handleInventory)
	s.mux.HandleFunc("GET /v1/docker/inspect", s.handleDockerInspect)
	s.mux.HandleFunc("GET /v1/runtime/compare", s.handleRuntimeCompare)
	s.mux.HandleFunc("POST /v1/plan/dashboard", s.handlePlanDashboard)
	s.mux.HandleFunc("POST /v1/plan/amud", s.handlePlanAMUD)
	s.mux.HandleFunc("POST /v1/plan/tz", s.handlePlanTZ)
	s.mux.HandleFunc("POST /v1/plan/recreate", s.handlePlanRecreate)
	s.mux.HandleFunc("POST /v1/apply/dashboard", s.handleApplyDashboard)
	s.mux.HandleFunc("POST /v1/apply/amud", s.handleApplyAMUD)
	s.mux.HandleFunc("POST /v1/apply/tz", s.handleApplyTZ)
	s.mux.HandleFunc("POST /v1/apply/recreate", s.handleApplyRecreate)
	s.mux.HandleFunc("POST /v1/restore/xml", s.handleRestoreXML)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"templates_dir":     s.config.TemplatesDir,
		"write_enabled":     true,
		"approval_required": s.config.RequireApprovalToken,
	})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, CapabilitiesResponse{
		Version:    "0.1.6",
		ToolSafety: "named-actions-only; no raw shell; plan-hash-confirmed writes",
		Capabilities: []CapabilityRecord{
			{
				ID:          "inventory",
				Status:      "implemented",
				Access:      "read",
				Description: "Read DockerMan XML templates, metadata, variables, ports, paths, labels and template risk findings.",
				Actions:     []string{"read"},
			},
			{
				ID:          "docker-runtime-inspect",
				Status:      "implemented",
				Access:      "read",
				Description: "Read normalized Docker runtime state through configured Docker API access or inspect snapshots.",
				Actions:     []string{"read"},
			},
			{
				ID:          "dashboard-config",
				Status:      "implemented-first-provider",
				Access:      "plan-apply",
				Description: "Plan and apply dashboard configuration through provider adapters. AMUD DockerMan labels are the first adapter.",
				Providers:   []string{planner.DashboardProviderAMUD},
				Actions:     []string{"plan", "diff", "apply"},
			},
			{
				ID:          "template-env",
				Status:      "implemented",
				Access:      "plan-apply",
				Description: "Plan and apply safe DockerMan XML environment variable edits such as TZ.",
				Actions:     []string{"plan", "diff", "apply"},
			},
			{
				ID:          "docker-recreate",
				Status:      "implemented",
				Access:      "plan-apply",
				Description: "Plan and apply DockerMan recreate operations through whitelisted rebuild_container calls.",
				Actions:     []string{"plan", "apply"},
			},
			{
				ID:          "xml-restore",
				Status:      "implemented",
				Access:      "apply",
				Description: "Restore DockerMan XML templates from verified backups.",
				Actions:     []string{"apply"},
			},
			{
				ID:          "community-applications",
				Status:      "planned",
				Access:      "none-yet",
				Description: "Future provider for installing Unraid Community Applications while preserving DockerMan/Apps metadata.",
				Actions:     []string{"search", "plan", "diff", "apply"},
			},
			{
				ID:          "general-unraid-control",
				Status:      "planned",
				Access:      "none-yet",
				Description: "Future safe modules for shares, VMs, plugins, array status and broader Unraid administration.",
				Actions:     []string{"read", "plan", "apply"},
			},
		},
	})
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	templates, err := s.loadTemplates()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, BuildInventoryResponse(templates))
}

func (s *Server) handleDockerInspect(w http.ResponseWriter, r *http.Request) {
	containers, err := s.loadRuntime(r.Context(), "")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleRuntimeCompare(w http.ResponseWriter, r *http.Request) {
	templates, err := s.loadTemplates()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	containers, err := s.loadRuntime(r.Context(), r.URL.Query().Get("inspect_path"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, compare.RuntimeVsTemplates(templates, containers))
}

func (s *Server) handlePlanAMUD(w http.ResponseWriter, r *http.Request) {
	var request AMUDPlanRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	templates, err := s.loadTemplates()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	runtimeFilter := request.RuntimeFilter
	if runtimeFilter == "" && s.hasRuntimeSource(request.InspectPath) {
		runtimeFilter = "running"
	}
	runtimeFilter, err = planner.NormalizeRuntimeFilter(runtimeFilter)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if runtimeFilter != "templates" {
		runtime, err := s.loadRuntime(r.Context(), request.InspectPath)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		templates, err = planner.FilterTemplatesByRuntime(templates, runtime, runtimeFilter)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	localHost := request.LocalHost
	if localHost == "" {
		localHost = s.config.LocalHost
	}
	plan := planner.BuildAMUDPlan(templates, planner.AMUDOptions{
		LocalHost:        localHost,
		URLMode:          request.URLMode,
		CloudflareDomain: request.CloudflareDomain,
		CloudflareRoutes: request.CloudflareRoutes,
		Names:            nameSet(request.Containers),
		ExcludedNames:    nameSet(request.ExcludeContainers),
		IncludePortOnly:  request.IncludePortOnly,
		RuntimeFilter:    runtimeFilter,
	})
	response := AMUDPlanResponse{Plan: plan}
	if request.IncludeDiffs {
		diffs, err := BuildAMUDDiffs(plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.Diffs = diffs
	}
	if request.SavePlan {
		path, err := s.savePlan("amud", plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.PlanPath = path
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handlePlanDashboard(w http.ResponseWriter, r *http.Request) {
	var request DashboardPlanRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	templates, err := s.loadTemplates()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	runtimeFilter := request.RuntimeFilter
	if runtimeFilter == "" && s.hasRuntimeSource(request.InspectPath) {
		runtimeFilter = "running"
	}
	runtimeFilter, err = planner.NormalizeRuntimeFilter(runtimeFilter)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if runtimeFilter != "templates" {
		runtime, err := s.loadRuntime(r.Context(), request.InspectPath)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		templates, err = planner.FilterTemplatesByRuntime(templates, runtime, runtimeFilter)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	localHost := request.LocalHost
	if localHost == "" {
		localHost = s.config.LocalHost
	}
	plan, err := planner.BuildDashboardPlan(templates, planner.DashboardOptions{
		Provider:         request.Provider,
		LocalHost:        localHost,
		URLMode:          request.URLMode,
		CloudflareDomain: request.CloudflareDomain,
		CloudflareRoutes: request.CloudflareRoutes,
		Names:            nameSet(request.Containers),
		ExcludedNames:    nameSet(request.ExcludeContainers),
		IncludePortOnly:  request.IncludePortOnly,
		RuntimeFilter:    runtimeFilter,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	response := DashboardPlanResponse{Plan: plan}
	if request.IncludeDiffs {
		diffs, err := BuildDashboardDiffs(plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.Diffs = diffs
	}
	if request.SavePlan {
		path, err := s.savePlan("dashboard", plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.PlanPath = path
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handlePlanTZ(w http.ResponseWriter, r *http.Request) {
	var request TZPlanRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if request.Timezone == "" {
		request.Timezone = "Europe/Prague"
	}
	templates, err := s.loadTemplates()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	plan := planner.BuildTZPlan(templates, planner.TZOptions{
		Timezone:         request.Timezone,
		Names:            nameSet(request.Containers),
		IncludeUnchanged: request.IncludeUnchanged,
	})
	response := TZPlanResponse{Plan: plan}
	if request.IncludeDiffs {
		diffs, err := BuildTZDiffs(plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.Diffs = diffs
	}
	if request.SavePlan {
		path, err := s.savePlan("tz", plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.PlanPath = path
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handlePlanRecreate(w http.ResponseWriter, r *http.Request) {
	var request RecreatePlanRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	templates, err := s.loadTemplates()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	containers, err := s.loadRuntime(r.Context(), request.InspectPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	plan := lifecycle.BuildRecreatePlan(templates, containers, lifecycle.Options{
		IncludeAll: request.All,
		Names:      nameSet(request.Containers),
	})
	response := RecreatePlanResponse{Plan: plan}
	if request.SavePlan {
		path, err := s.savePlan("recreate", plan)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		response.PlanPath = path
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleApplyAMUD(w http.ResponseWriter, r *http.Request) {
	var request ApplyAMUDRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	plan, err := resolveAMUDPlan(request)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.consumeApproval(plan.PlanHash, request.ApprovalToken); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	report, err := executor.ApplyAMUDPlan(plan, executor.AMUDApplyOptions{
		ConfirmPlanHash: request.ConfirmPlanHash,
		BackupDir:       s.config.BackupDir,
		AuditDir:        s.config.AuditDir,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleApplyDashboard(w http.ResponseWriter, r *http.Request) {
	var request ApplyDashboardRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	plan, err := resolveDashboardPlan(request)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.consumeApproval(plan.PlanHash, request.ApprovalToken); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	report, err := executor.ApplyDashboardPlan(plan, executor.DashboardApplyOptions{
		ConfirmPlanHash: request.ConfirmPlanHash,
		BackupDir:       s.config.BackupDir,
		AuditDir:        s.config.AuditDir,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleApplyTZ(w http.ResponseWriter, r *http.Request) {
	var request ApplyTZRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	plan, err := resolveTZPlan(request)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.consumeApproval(plan.PlanHash, request.ApprovalToken); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	report, err := executor.ApplyTZPlan(plan, executor.TZApplyOptions{
		ConfirmPlanHash: request.ConfirmPlanHash,
		BackupDir:       s.config.BackupDir,
		AuditDir:        s.config.AuditDir,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleApplyRecreate(w http.ResponseWriter, r *http.Request) {
	var request ApplyRecreateRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	plan, err := resolveRecreatePlan(request)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.consumeApproval(plan.PlanHash, request.ApprovalToken); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	runtime, err := s.runtimeController()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	report, err := executor.ApplyRecreatePlan(r.Context(), plan, executor.RecreateApplyOptions{
		ConfirmPlanHash: request.ConfirmPlanHash,
		AuditDir:        s.config.AuditDir,
		Runtime:         runtime,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) consumeApproval(planHash string, token string) error {
	if !s.config.RequireApprovalToken {
		return nil
	}
	return approval.Consume(s.config.ApprovalsDir, planHash, token, time.Time{})
}

func (s *Server) handleRestoreXML(w http.ResponseWriter, r *http.Request) {
	var request RestoreXMLRequest
	if err := s.readJSON(r, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	report, err := executor.RestoreXMLBackup(executor.RestoreXMLOptions{
		BackupPath:          request.BackupPath,
		TargetPath:          request.TargetPath,
		ConfirmBackupSHA256: request.ConfirmBackupSHA256,
		PreRestoreBackupDir: filepath.Join(s.config.BackupDir, "pre-restore"),
		AuditDir:            s.config.AuditDir,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) loadTemplates() ([]dockerxml.Template, error) {
	return dockerxml.LoadTemplates(s.config.TemplatesDir)
}

func (s *Server) loadRuntime(ctx context.Context, inspectPath string) ([]dockerinspect.Container, error) {
	if inspectPath != "" {
		return dockerinspect.LoadPath(inspectPath)
	}
	if s.config.DockerSocket != "" {
		return dockerapi.NewUnixSocketClient(s.config.DockerSocket).InspectAll(ctx)
	}
	if s.config.DockerHost != "" {
		return dockerapi.NewHTTPClient(s.config.DockerHost).InspectAll(ctx)
	}
	return nil, errors.New("runtime source is not configured")
}

func (s *Server) runtimeController() (*dockerapi.Client, error) {
	if s.config.DockerSocket != "" {
		return dockerapi.NewUnixSocketClient(s.config.DockerSocket), nil
	}
	if s.config.DockerHost != "" {
		return dockerapi.NewHTTPClient(s.config.DockerHost), nil
	}
	return nil, errors.New("Docker runtime source is not configured")
}

func (s *Server) hasRuntimeSource(inspectPath string) bool {
	return inspectPath != "" || s.config.DockerSocket != "" || s.config.DockerHost != ""
}

func (s *Server) savePlan(kind string, value any) (string, error) {
	if err := os.MkdirAll(s.config.PlansDir, 0o700); err != nil {
		return "", err
	}
	hash := planHash(value)
	path := filepath.Join(s.config.PlansDir, fmt.Sprintf("%s_%s_%s.json", time.Now().UTC().Format("20060102T150405Z"), kind, hash))
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func BuildInventoryResponse(templates []dockerxml.Template) InventoryResponse {
	response := InventoryResponse{
		WriteEnabled:  true,
		TemplateCount: len(templates),
		Containers:    make([]ContainerRecord, 0, len(templates)),
	}
	for _, template := range templates {
		response.Containers = append(response.Containers, ContainerRecord{
			SourcePath:   template.SourcePath,
			Version:      template.Version,
			Name:         template.Name,
			Repository:   template.Repository,
			Registry:     template.Registry,
			Network:      template.Network,
			Privileged:   template.Privileged,
			WebUI:        template.WebUI,
			TemplateURL:  template.TemplateURL,
			Icon:         template.Icon,
			ExtraParams:  template.ExtraParams,
			Ports:        template.Ports(),
			Paths:        template.Paths(),
			Variables:    template.Variables(),
			Labels:       template.Labels(),
			RiskFindings: risk.AnalyzeTemplate(template),
		})
	}
	return response
}

func BuildAMUDDiffs(plan planner.AMUDPlan) ([]DiffRecord, error) {
	records := make([]DiffRecord, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		originalBytes, err := os.ReadFile(entry.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.SourcePath, err)
		}
		patch, err := xmlpatch.ApplyAMUDLabels(string(originalBytes), entry.ProposedLabels)
		if err != nil {
			return nil, fmt.Errorf("build AMUD XML patch for %s: %w", entry.SourcePath, err)
		}
		records = append(records, DiffRecord{
			Container:   entry.Container,
			SourcePath:  entry.SourcePath,
			Changed:     patch.Changed,
			Operations:  patch.Ops,
			UnifiedDiff: textdiff.Unified(entry.SourcePath, entry.SourcePath+" (candidate)", patch.Original, patch.Modified, 3),
		})
	}
	return records, nil
}

func BuildDashboardDiffs(plan planner.DashboardPlan) ([]DiffRecord, error) {
	provider, err := planner.NormalizeDashboardProvider(plan.Provider)
	if err != nil {
		return nil, err
	}
	switch provider {
	case planner.DashboardProviderAMUD:
		plan.Provider = provider
		amudPlan, err := planner.AMUDPlanFromDashboardPlan(plan)
		if err != nil {
			return nil, err
		}
		return BuildAMUDDiffs(amudPlan)
	default:
		return nil, fmt.Errorf("unsupported dashboard provider: %s", provider)
	}
}

func BuildTZDiffs(plan planner.TZPlan) ([]DiffRecord, error) {
	records := make([]DiffRecord, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		originalBytes, err := os.ReadFile(entry.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.SourcePath, err)
		}
		patch, err := xmlpatch.ApplyVariable(string(originalBytes), "TZ", entry.ProposedValue, "TZ")
		if err != nil {
			return nil, fmt.Errorf("build TZ XML patch for %s: %w", entry.SourcePath, err)
		}
		records = append(records, DiffRecord{
			Container:   entry.Container,
			SourcePath:  entry.SourcePath,
			Changed:     patch.Changed,
			Operations:  patch.Ops,
			UnifiedDiff: textdiff.Unified(entry.SourcePath, entry.SourcePath+" (candidate)", patch.Original, patch.Modified, 3),
		})
	}
	return records, nil
}

func resolveAMUDPlan(request ApplyAMUDRequest) (planner.AMUDPlan, error) {
	if request.Plan != nil {
		return *request.Plan, nil
	}
	if request.PlanPath == "" {
		return planner.AMUDPlan{}, errors.New("plan or plan_path is required")
	}
	return executor.ReadAMUDPlanFile(request.PlanPath)
}

func resolveDashboardPlan(request ApplyDashboardRequest) (planner.DashboardPlan, error) {
	if request.Plan != nil {
		return *request.Plan, nil
	}
	if request.PlanPath == "" {
		return planner.DashboardPlan{}, errors.New("plan or plan_path is required")
	}
	return executor.ReadDashboardPlanFile(request.PlanPath)
}

func resolveTZPlan(request ApplyTZRequest) (planner.TZPlan, error) {
	if request.Plan != nil {
		return *request.Plan, nil
	}
	if request.PlanPath == "" {
		return planner.TZPlan{}, errors.New("plan or plan_path is required")
	}
	return executor.ReadTZPlanFile(request.PlanPath)
}

func resolveRecreatePlan(request ApplyRecreateRequest) (lifecycle.RecreatePlan, error) {
	if request.Plan != nil {
		return *request.Plan, nil
	}
	if request.PlanPath == "" {
		return lifecycle.RecreatePlan{}, errors.New("plan or plan_path is required")
	}
	return executor.ReadRecreatePlanFile(request.PlanPath)
}

func (s *Server) readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 10<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	s.logger.Warn("request failed", "status", status, "error", err)
	s.writeJSON(w, status, ErrorResponse{Error: err.Error()})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := r.Header.Get("X-Unraid-AI-Key")
		if token == "" {
			auth := r.Header.Get("Authorization")
			token = strings.TrimPrefix(auth, "Bearer ")
		}
		if token != s.config.APIKey {
			s.writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func nameSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[strings.ToLower(value)] = true
		}
	}
	return set
}

func planHash(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}
