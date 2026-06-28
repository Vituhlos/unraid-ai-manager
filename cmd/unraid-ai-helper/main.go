package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"unraid-ai-manager/internal/server"
)

func main() {
	listen := flag.String("listen", envOr("UNRAID_AI_LISTEN", "127.0.0.1:37231"), "HTTP listen address.")
	templatesDir := flag.String("templates", envOr("UNRAID_AI_TEMPLATES", "/boot/config/plugins/dockerMan/templates-user"), "DockerMan templates-user directory.")
	backupDir := flag.String("backup-dir", envOr("UNRAID_AI_BACKUP_DIR", "/mnt/user/appdata/unraid-ai-manager/backups"), "XML backup directory.")
	auditDir := flag.String("audit-dir", envOr("UNRAID_AI_AUDIT_DIR", "/mnt/user/appdata/unraid-ai-manager/audit"), "Audit log directory.")
	plansDir := flag.String("plans-dir", envOr("UNRAID_AI_PLANS_DIR", "/mnt/user/appdata/unraid-ai-manager/plans"), "Saved plans directory.")
	approvalsDir := flag.String("approvals-dir", envOr("UNRAID_AI_APPROVALS_DIR", "/mnt/user/appdata/unraid-ai-manager/approvals"), "Local approval records directory.")
	dockerSocket := flag.String("docker-socket", envOr("UNRAID_AI_DOCKER_SOCKET", "/var/run/docker.sock"), "Docker unix socket path.")
	dockerHost := flag.String("docker-host", envOr("UNRAID_AI_DOCKER_HOST", ""), "Docker HTTP API endpoint.")
	localHost := flag.String("local-host", envOr("UNRAID_AI_LOCAL_HOST", ""), "Default local host/IP for URL planning.")
	apiKey := flag.String("api-key", envOr("UNRAID_AI_API_KEY", ""), "Optional API key required via X-Unraid-AI-Key or Bearer token.")
	requireApproval := flag.Bool("require-approval-token", envBool("UNRAID_AI_REQUIRE_APPROVAL_TOKEN", true), "Require local approval token for apply endpoints.")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	helper, err := server.New(server.Config{
		ListenAddr:           *listen,
		TemplatesDir:         *templatesDir,
		BackupDir:            *backupDir,
		AuditDir:             *auditDir,
		PlansDir:             *plansDir,
		ApprovalsDir:         *approvalsDir,
		DockerSocket:         *dockerSocket,
		DockerHost:           *dockerHost,
		LocalHost:            *localHost,
		APIKey:               *apiKey,
		RequireApprovalToken: *requireApproval,
		Logger:               logger,
	})
	if err != nil {
		logger.Error("helper init failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := helper.ListenAndServe(ctx); err != nil {
		logger.Error("helper failed", "error", err)
		os.Exit(1)
	}
}

func envOr(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	switch value {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}
