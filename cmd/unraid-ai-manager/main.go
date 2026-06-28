package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
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

type inventoryPayload struct {
	WriteEnabled  bool              `json:"write_enabled"`
	TemplateCount int               `json:"template_count"`
	Containers    []containerRecord `json:"containers"`
}

type containerRecord struct {
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

type amudPlanWithDiffs struct {
	Plan  planner.AMUDPlan `json:"plan"`
	Diffs []diffRecord     `json:"diffs"`
}

type tzPlanWithDiffs struct {
	Plan  planner.TZPlan `json:"plan"`
	Diffs []diffRecord   `json:"diffs"`
}

type diffRecord struct {
	Container   string               `json:"container"`
	SourcePath  string               `json:"source_path"`
	Changed     bool                 `json:"changed"`
	Operations  []xmlpatch.Operation `json:"operations"`
	UnifiedDiff string               `json:"unified_diff"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("missing command")
	}

	switch args[0] {
	case "inventory":
		return runInventory(args[1:])
	case "inspect-json":
		return runInspectJSON(args[1:])
	case "inspect-docker":
		return runInspectDocker(args[1:])
	case "compare-runtime":
		return runCompareRuntime(args[1:])
	case "plan-recreate":
		return runPlanRecreate(args[1:])
	case "plan-amud":
		return runPlanAMUD(args[1:])
	case "plan-tz":
		return runPlanTZ(args[1:])
	case "approve-plan":
		return runApprovePlan(args[1:])
	case "apply-amud-plan":
		return runApplyAMUDPlan(args[1:])
	case "apply-tz-plan":
		return runApplyTZPlan(args[1:])
	case "apply-recreate-plan":
		return runApplyRecreatePlan(args[1:])
	case "restore-xml-backup":
		return runRestoreXMLBackup(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runInventory(args []string) error {
	flags := flag.NewFlagSet("inventory", flag.ContinueOnError)
	templatesPath := flags.String("templates", "", "Path to templates-user directory or one XML file.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *templatesPath == "" {
		return errors.New("--templates is required")
	}

	templates, err := dockerxml.LoadTemplates(*templatesPath)
	if err != nil {
		return err
	}
	payload := buildInventoryPayload(templates)
	if *jsonOutput {
		return printJSON(payload)
	}
	printInventory(payload)
	return nil
}

func runApprovePlan(args []string) error {
	flags := flag.NewFlagSet("approve-plan", flag.ContinueOnError)
	planPath := flags.String("plan", "", "Path to exported plan JSON.")
	approvalsDir := flags.String("approvals-dir", "", "Directory for approval records.")
	purpose := flags.String("purpose", "apply", "Approval purpose, e.g. amud, tz, recreate.")
	ttl := flags.Duration("ttl", 15*time.Minute, "Approval token TTL.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *planPath == "" {
		return errors.New("--plan is required")
	}
	if *approvalsDir == "" {
		return errors.New("--approvals-dir is required")
	}
	planHash, err := approval.ExtractPlanHashFromFile(*planPath)
	if err != nil {
		return err
	}
	grant, err := approval.Grant(*approvalsDir, planHash, *purpose, *ttl, time.Time{})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(grant)
	}
	fmt.Println("Approval granted")
	fmt.Printf("Plan hash: %s\n", grant.PlanHash)
	fmt.Printf("Purpose:   %s\n", grant.Purpose)
	fmt.Printf("Expires:   %s\n", grant.ExpiresAt)
	fmt.Printf("Record:    %s\n", grant.Path)
	fmt.Println()
	fmt.Printf("Approval token: %s\n", grant.Token)
	return nil
}

func runInspectDocker(args []string) error {
	flags := flag.NewFlagSet("inspect-docker", flag.ContinueOnError)
	dockerSocket := flags.String("docker-socket", "", "Docker unix socket path, e.g. /var/run/docker.sock.")
	dockerHost := flags.String("docker-host", "", "Docker HTTP API endpoint. Use only for trusted local/proxy endpoints.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	containers, err := loadRuntimeContainers("", *dockerSocket, *dockerHost)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(containers)
	}
	printInspectInventory(containers)
	return nil
}

func runInspectJSON(args []string) error {
	flags := flag.NewFlagSet("inspect-json", flag.ContinueOnError)
	inspectPath := flags.String("inspect", "", "Path to docker inspect JSON file or directory.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inspectPath == "" {
		return errors.New("--inspect is required")
	}

	containers, err := dockerinspect.LoadPath(*inspectPath)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(containers)
	}
	printInspectInventory(containers)
	return nil
}

func runCompareRuntime(args []string) error {
	flags := flag.NewFlagSet("compare-runtime", flag.ContinueOnError)
	templatesPath := flags.String("templates", "", "Path to templates-user directory or one XML file.")
	inspectPath := flags.String("inspect", "", "Path to docker inspect JSON file or directory.")
	dockerSocket := flags.String("docker-socket", "", "Docker unix socket path, e.g. /var/run/docker.sock.")
	dockerHost := flags.String("docker-host", "", "Docker HTTP API endpoint. Use only for trusted local/proxy endpoints.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *templatesPath == "" {
		return errors.New("--templates is required")
	}

	templates, err := dockerxml.LoadTemplates(*templatesPath)
	if err != nil {
		return err
	}
	containers, err := loadRuntimeContainers(*inspectPath, *dockerSocket, *dockerHost)
	if err != nil {
		return err
	}
	report := compare.RuntimeVsTemplates(templates, containers)
	if *jsonOutput {
		return printJSON(report)
	}
	printRuntimeCompare(report)
	return nil
}

func runPlanRecreate(args []string) error {
	flags := flag.NewFlagSet("plan-recreate", flag.ContinueOnError)
	templatesPath := flags.String("templates", "", "Path to templates-user directory or one XML file.")
	inspectPath := flags.String("inspect", "", "Path to docker inspect JSON file or directory.")
	dockerSocket := flags.String("docker-socket", "", "Docker unix socket path, e.g. /var/run/docker.sock.")
	dockerHost := flags.String("docker-host", "", "Docker HTTP API endpoint. Use only for trusted local/proxy endpoints.")
	includeAll := flags.Bool("all", false, "Include matched containers even when no recreate reason is detected.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	outPath := flags.String("out", "", "Write recreate plan JSON to this path.")
	var names nameFlags
	flags.Var(&names, "container", "Limit plan to a container name. Can be repeated.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *templatesPath == "" {
		return errors.New("--templates is required")
	}

	templates, err := dockerxml.LoadTemplates(*templatesPath)
	if err != nil {
		return err
	}
	containers, err := loadRuntimeContainers(*inspectPath, *dockerSocket, *dockerHost)
	if err != nil {
		return err
	}
	plan := lifecycle.BuildRecreatePlan(templates, containers, lifecycle.Options{
		IncludeAll: *includeAll,
		Names:      names.Map,
	})
	if *outPath != "" {
		if err := writeJSONFile(*outPath, plan); err != nil {
			return err
		}
	}
	if *jsonOutput {
		return printJSON(plan)
	}
	printRecreatePlan(plan)
	if *outPath != "" {
		fmt.Printf("Recreate plan JSON written to: %s\n", *outPath)
	}
	return nil
}

func runPlanAMUD(args []string) error {
	flags := flag.NewFlagSet("plan-amud", flag.ContinueOnError)
	templatesPath := flags.String("templates", "", "Path to templates-user directory or one XML file.")
	localHost := flags.String("local-host", "", "Local Unraid host/IP for local AMUD URLs.")
	urlMode := flags.String("url-mode", "local", "URL mode: local, cloudflare, hybrid.")
	cloudflareDomain := flags.String("cloudflare-domain", "", "Cloudflare base domain, e.g. example.com.")
	includePortOnly := flags.Bool("include-port-only", false, "Also include templates without WebUI that only expose a TCP port.")
	runtimeFilter := flags.String("runtime-filter", "templates", "Runtime filter: templates, existing, or running.")
	inspectPath := flags.String("inspect", "", "Optional docker inspect JSON file/directory for runtime filtering.")
	dockerSocket := flags.String("docker-socket", "", "Docker unix socket path for runtime filtering, e.g. /var/run/docker.sock.")
	dockerHost := flags.String("docker-host", "", "Docker HTTP API endpoint for runtime filtering. Use only for trusted local/proxy endpoints.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	diffOutput := flags.Bool("diff", false, "Print candidate DockerMan XML unified diff. Read-only; does not write files.")
	outPath := flags.String("out", "", "Write plan JSON to this path. Read-only for Unraid templates.")
	var routes routeFlags
	var names nameFlags
	var excludes nameFlags
	flags.Var(&routes, "route", "Cloudflare mapping NAME=SUBDOMAIN_OR_URL. Can be repeated.")
	flags.Var(&names, "container", "Limit AMUD plan to a container name. Can be repeated.")
	flags.Var(&excludes, "exclude", "Exclude a container name from the AMUD plan. Can be repeated.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *templatesPath == "" {
		return errors.New("--templates is required")
	}
	if *localHost == "" {
		return errors.New("--local-host is required")
	}
	if *urlMode != "local" && *urlMode != "cloudflare" && *urlMode != "hybrid" {
		return errors.New("--url-mode must be local, cloudflare, or hybrid")
	}

	templates, err := dockerxml.LoadTemplates(*templatesPath)
	if err != nil {
		return err
	}
	normalizedRuntimeFilter, err := planner.NormalizeRuntimeFilter(*runtimeFilter)
	if err != nil {
		return err
	}
	if normalizedRuntimeFilter != "templates" {
		runtime, err := loadRuntimeContainers(*inspectPath, *dockerSocket, *dockerHost)
		if err != nil {
			return err
		}
		templates, err = planner.FilterTemplatesByRuntime(templates, runtime, normalizedRuntimeFilter)
		if err != nil {
			return err
		}
	}
	plan := planner.BuildAMUDPlan(templates, planner.AMUDOptions{
		LocalHost:        *localHost,
		URLMode:          *urlMode,
		CloudflareDomain: *cloudflareDomain,
		CloudflareRoutes: routes.Map,
		Names:            names.Map,
		ExcludedNames:    excludes.Map,
		IncludePortOnly:  *includePortOnly,
		RuntimeFilter:    normalizedRuntimeFilter,
	})
	if *diffOutput {
		diffs, err := buildAMUDDiffs(plan)
		if err != nil {
			return err
		}
		if *outPath != "" {
			if err := writeJSONFile(*outPath, amudPlanWithDiffs{Plan: plan, Diffs: diffs}); err != nil {
				return err
			}
		}
		if *jsonOutput {
			return printJSON(amudPlanWithDiffs{Plan: plan, Diffs: diffs})
		}
		printAMUDPlan(plan, false)
		printAMUDDiffs(diffs)
		if *outPath != "" {
			fmt.Printf("Plan JSON written to: %s\n", *outPath)
		}
		return nil
	}
	if *outPath != "" {
		if err := writeJSONFile(*outPath, plan); err != nil {
			return err
		}
	}
	if *jsonOutput {
		return printJSON(plan)
	}
	printAMUDPlan(plan, true)
	if *outPath != "" {
		fmt.Printf("Plan JSON written to: %s\n", *outPath)
	}
	return nil
}

func runPlanTZ(args []string) error {
	flags := flag.NewFlagSet("plan-tz", flag.ContinueOnError)
	templatesPath := flags.String("templates", "", "Path to templates-user directory or one XML file.")
	timezone := flags.String("tz", "Europe/Prague", "Timezone value to set in TZ env variable.")
	includeUnchanged := flags.Bool("include-unchanged", false, "Include templates where TZ already matches.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON.")
	diffOutput := flags.Bool("diff", false, "Print candidate DockerMan XML unified diff. Read-only; does not write files.")
	outPath := flags.String("out", "", "Write TZ plan JSON to this path.")
	var names nameFlags
	flags.Var(&names, "container", "Limit plan to a container name. Can be repeated.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *templatesPath == "" {
		return errors.New("--templates is required")
	}
	if *timezone == "" {
		return errors.New("--tz cannot be empty")
	}

	templates, err := dockerxml.LoadTemplates(*templatesPath)
	if err != nil {
		return err
	}
	plan := planner.BuildTZPlan(templates, planner.TZOptions{
		Timezone:         *timezone,
		Names:            names.Map,
		IncludeUnchanged: *includeUnchanged,
	})
	if *diffOutput {
		diffs, err := buildTZDiffs(plan)
		if err != nil {
			return err
		}
		if *outPath != "" {
			if err := writeJSONFile(*outPath, tzPlanWithDiffs{Plan: plan, Diffs: diffs}); err != nil {
				return err
			}
		}
		if *jsonOutput {
			return printJSON(tzPlanWithDiffs{Plan: plan, Diffs: diffs})
		}
		printTZPlan(plan, false)
		printTZDiffs(diffs)
		if *outPath != "" {
			fmt.Printf("TZ plan JSON written to: %s\n", *outPath)
		}
		return nil
	}
	if *outPath != "" {
		if err := writeJSONFile(*outPath, plan); err != nil {
			return err
		}
	}
	if *jsonOutput {
		return printJSON(plan)
	}
	printTZPlan(plan, true)
	if *outPath != "" {
		fmt.Printf("TZ plan JSON written to: %s\n", *outPath)
	}
	return nil
}

func runApplyAMUDPlan(args []string) error {
	flags := flag.NewFlagSet("apply-amud-plan", flag.ContinueOnError)
	planPath := flags.String("plan", "", "Path to exported AMUD plan JSON.")
	confirmPlanHash := flags.String("confirm-plan-hash", "", "Exact plan hash required to apply.")
	backupDir := flags.String("backup-dir", "", "Directory for XML backups. Must not be inside templates-user.")
	auditDir := flags.String("audit-dir", "", "Directory for audit JSON logs. Must not be inside templates-user.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON report.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *planPath == "" {
		return errors.New("--plan is required")
	}

	plan, err := executor.ReadAMUDPlanFile(*planPath)
	if err != nil {
		return fmt.Errorf("read AMUD plan: %w", err)
	}
	report, err := executor.ApplyAMUDPlan(plan, executor.AMUDApplyOptions{
		ConfirmPlanHash: *confirmPlanHash,
		BackupDir:       *backupDir,
		AuditDir:        *auditDir,
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(report)
	}
	printApplyReport(report)
	return nil
}

func runApplyTZPlan(args []string) error {
	flags := flag.NewFlagSet("apply-tz-plan", flag.ContinueOnError)
	planPath := flags.String("plan", "", "Path to exported TZ plan JSON.")
	confirmPlanHash := flags.String("confirm-plan-hash", "", "Exact plan hash required to apply.")
	backupDir := flags.String("backup-dir", "", "Directory for XML backups. Must not be inside templates-user.")
	auditDir := flags.String("audit-dir", "", "Directory for audit JSON logs. Must not be inside templates-user.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON report.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *planPath == "" {
		return errors.New("--plan is required")
	}

	plan, err := executor.ReadTZPlanFile(*planPath)
	if err != nil {
		return fmt.Errorf("read TZ plan: %w", err)
	}
	report, err := executor.ApplyTZPlan(plan, executor.TZApplyOptions{
		ConfirmPlanHash: *confirmPlanHash,
		BackupDir:       *backupDir,
		AuditDir:        *auditDir,
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(report)
	}
	printTZApplyReport(report)
	return nil
}

func runApplyRecreatePlan(args []string) error {
	flags := flag.NewFlagSet("apply-recreate-plan", flag.ContinueOnError)
	planPath := flags.String("plan", "", "Path to exported recreate plan JSON.")
	confirmPlanHash := flags.String("confirm-plan-hash", "", "Exact plan hash required to apply.")
	auditDir := flags.String("audit-dir", "", "Directory for recreate audit JSON logs.")
	dockerSocket := flags.String("docker-socket", "", "Docker unix socket path, e.g. /var/run/docker.sock.")
	dockerHost := flags.String("docker-host", "", "Docker HTTP API endpoint. Use only for trusted local/proxy endpoints.")
	rebuildScript := flags.String("rebuild-script", executor.DefaultDockerManRebuildScript, "Absolute path to DockerMan rebuild_container script.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON report.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *planPath == "" {
		return errors.New("--plan is required")
	}
	if (*dockerSocket == "") == (*dockerHost == "") {
		return errors.New("exactly one runtime source is required: --docker-socket or --docker-host")
	}

	plan, err := executor.ReadRecreatePlanFile(*planPath)
	if err != nil {
		return fmt.Errorf("read recreate plan: %w", err)
	}
	var runtime *dockerapi.Client
	if *dockerSocket != "" {
		runtime = dockerapi.NewUnixSocketClient(*dockerSocket)
	} else {
		runtime = dockerapi.NewHTTPClient(*dockerHost)
	}
	report, err := executor.ApplyRecreatePlan(context.Background(), plan, executor.RecreateApplyOptions{
		ConfirmPlanHash: *confirmPlanHash,
		AuditDir:        *auditDir,
		RebuildScript:   *rebuildScript,
		Runtime:         runtime,
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(report)
	}
	printRecreateApplyReport(report)
	return nil
}

func runRestoreXMLBackup(args []string) error {
	flags := flag.NewFlagSet("restore-xml-backup", flag.ContinueOnError)
	backupPath := flags.String("backup", "", "Path to XML backup to restore.")
	targetPath := flags.String("target", "", "Path to DockerMan XML template to replace.")
	confirmBackupSHA := flags.String("confirm-backup-sha256", "", "Exact SHA256 of backup file required to restore.")
	preRestoreBackupDir := flags.String("pre-restore-backup-dir", "", "Directory for backing up current target before restore.")
	auditDir := flags.String("audit-dir", "", "Directory for restore audit JSON logs. Must not be inside templates-user.")
	jsonOutput := flags.Bool("json", false, "Print machine-readable JSON report.")
	if err := flags.Parse(args); err != nil {
		return err
	}

	report, err := executor.RestoreXMLBackup(executor.RestoreXMLOptions{
		BackupPath:          *backupPath,
		TargetPath:          *targetPath,
		ConfirmBackupSHA256: *confirmBackupSHA,
		PreRestoreBackupDir: *preRestoreBackupDir,
		AuditDir:            *auditDir,
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return printJSON(report)
	}
	printRestoreReport(report)
	return nil
}

func loadRuntimeContainers(inspectPath string, dockerSocket string, dockerHost string) ([]dockerinspect.Container, error) {
	sourceCount := 0
	if inspectPath != "" {
		sourceCount++
	}
	if dockerSocket != "" {
		sourceCount++
	}
	if dockerHost != "" {
		sourceCount++
	}
	if sourceCount == 0 {
		return nil, errors.New("one runtime source is required: --inspect, --docker-socket, or --docker-host")
	}
	if sourceCount > 1 {
		return nil, errors.New("use only one runtime source: --inspect, --docker-socket, or --docker-host")
	}
	if inspectPath != "" {
		return dockerinspect.LoadPath(inspectPath)
	}

	var client *dockerapi.Client
	if dockerSocket != "" {
		client = dockerapi.NewUnixSocketClient(dockerSocket)
	} else {
		client = dockerapi.NewHTTPClient(dockerHost)
	}
	return client.InspectAll(context.Background())
}

func buildInventoryPayload(templates []dockerxml.Template) inventoryPayload {
	payload := inventoryPayload{
		WriteEnabled:  false,
		TemplateCount: len(templates),
		Containers:    make([]containerRecord, 0, len(templates)),
	}
	for _, template := range templates {
		payload.Containers = append(payload.Containers, containerRecord{
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
	return payload
}

func printInventory(payload inventoryPayload) {
	fmt.Println("Unraid AI Manager inventory (read-only)")
	fmt.Printf("Templates: %d\n\n", payload.TemplateCount)
	if len(payload.Containers) == 0 {
		fmt.Println("No templates found.")
		return
	}
	for _, container := range payload.Containers {
		fmt.Printf("- %s\n", container.Name)
		fmt.Printf("  Repository:  %s\n", valueOrDash(container.Repository))
		fmt.Printf("  Network:     %s\n", valueOrDash(container.Network))
		fmt.Printf("  WebUI:       %s\n", valueOrDash(container.WebUI))
		fmt.Printf("  Ports:       %s\n", portsSummary(container.Ports))
		fmt.Printf("  TemplateURL: %s\n", valueOrDash(container.TemplateURL))
		fmt.Printf("  Risks:       %s\n\n", riskSummary(container.RiskFindings))
	}
}

func printInspectInventory(containers []dockerinspect.Container) {
	fmt.Println("Docker inspect inventory (read-only)")
	fmt.Printf("Containers: %d\n\n", len(containers))
	if len(containers) == 0 {
		fmt.Println("No containers found.")
		return
	}
	for _, container := range containers {
		fmt.Printf("- %s\n", container.Name)
		fmt.Printf("  Image:   %s\n", valueOrDash(container.Image))
		fmt.Printf("  State:   %s\n", valueOrDash(container.State))
		fmt.Printf("  Network: %s\n", valueOrDash(container.NetworkMode))
		fmt.Printf("  Ports:   %s\n", runtimePortsSummary(container.Ports))
		fmt.Printf("  AMUD:    %s\n\n", amudLabelSummary(container.Labels))
	}
}

func printRuntimeCompare(report compare.Report) {
	fmt.Println("DockerMan XML vs Docker inspect comparison (read-only)")
	fmt.Printf("Templates: %d\n", report.TemplateCount)
	fmt.Printf("Runtime:   %d\n\n", report.RuntimeCount)

	for _, match := range report.Matches {
		fmt.Printf("- %s\n", match.Name)
		if len(match.Warnings) > 0 {
			fmt.Println("  Warnings:")
			for _, warning := range match.Warnings {
				fmt.Printf("    - %s\n", warning)
			}
		}
		if len(match.PortComparisons) > 0 {
			fmt.Println("  Ports:")
			for _, port := range match.PortComparisons {
				marker := "ok"
				if !port.Match {
					marker = "diff"
				}
				fmt.Printf("    - %s/%s template=%s runtime=%s [%s]\n", port.ContainerPort, port.Protocol, valueOrDash(port.TemplateHostPort), valueOrDash(port.RuntimeHostPort), marker)
			}
		}
		if len(match.LabelComparisons) > 0 {
			fmt.Println("  AMUD labels:")
			for _, label := range match.LabelComparisons {
				marker := "ok"
				if !label.Match {
					marker = "diff"
				}
				fmt.Printf("    - %s template=%s runtime=%s [%s]\n", label.Key, valueOrDash(label.TemplateValue), valueOrDash(label.RuntimeValue), marker)
			}
		}
		if len(match.EnvComparisons) > 0 {
			fmt.Println("  Env:")
			for _, env := range match.EnvComparisons {
				marker := "ok"
				if !env.Match {
					marker = "diff"
				}
				fmt.Printf("    - %s template=%s runtime=%s [%s]\n", env.Key, valueOrDash(env.TemplateValue), valueOrDash(env.RuntimeValue), marker)
			}
		}
		fmt.Println()
	}

	if len(report.MissingRuntime) > 0 {
		fmt.Println("Templates without matching runtime container:")
		for _, name := range report.MissingRuntime {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
	}
	if len(report.UnmatchedRuntime) > 0 {
		fmt.Println("Runtime containers without matching template:")
		for _, name := range report.UnmatchedRuntime {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println()
	}
}

func printRecreatePlan(plan lifecycle.RecreatePlan) {
	fmt.Println("Docker recreate plan (read-only)")
	fmt.Printf("Plan hash: %s\n", plan.PlanHash)
	fmt.Printf("Entries:   %d\n\n", len(plan.Entries))
	if len(plan.Entries) == 0 {
		fmt.Println("No containers need recreate according to the current comparison.")
		return
	}

	for _, entry := range plan.Entries {
		fmt.Printf("- %s\n", entry.Container)
		fmt.Printf("  State:    %s\n", valueOrDash(entry.State))
		fmt.Printf("  Template: %s\n", entry.TemplatePath)
		fmt.Println("  Reasons:")
		for _, reason := range entry.Reasons {
			fmt.Printf("    - %s\n", reason)
		}
		fmt.Println("  Preflight:")
		for _, check := range entry.Preflight {
			status := "ok"
			if !check.OK {
				status = "block"
			}
			fmt.Printf("    - [%s] %s: %s\n", status, check.Code, check.Message)
		}
		if len(entry.RiskFindings) > 0 {
			fmt.Println("  Template risk findings:")
			for _, finding := range entry.RiskFindings {
				if finding.Severity == "info" {
					continue
				}
				fmt.Printf("    - %s: %s\n", finding.Severity, finding.Message)
			}
		}
		fmt.Println()
	}
	fmt.Println("No containers were recreated.")
}

func printAMUDPlan(plan planner.AMUDPlan, printFooter bool) {
	fmt.Println("AMUD label plan (read-only)")
	fmt.Printf("Plan hash: %s\n", plan.PlanHash)
	fmt.Printf("URL mode:  %s\n", plan.URLMode)
	fmt.Printf("Local:     %s\n", plan.LocalHost)
	if plan.CloudflareDomain != "" {
		fmt.Printf("Cloudflare domain: %s\n", plan.CloudflareDomain)
	}
	fmt.Println()

	if len(plan.Entries) == 0 {
		fmt.Println("No web container candidates found.")
		return
	}

	for _, entry := range plan.Entries {
		fmt.Printf("- %s\n", entry.Container)
		fmt.Printf("  Detection: %s (%s)\n", entry.WebDetection.Confidence, entry.WebDetection.Reason)
		fmt.Printf("  URL:       %s\n", valueOrDash(entry.URL.URL))
		fmt.Println("  Labels:")
		for _, change := range entry.LabelChanges {
			prefix := "?"
			switch change.Action {
			case "add":
				prefix = "+"
			case "update":
				prefix = "~"
			case "unchanged":
				prefix = "="
			}
			current := ""
			if change.Current != nil {
				current = " (current: " + *change.Current + ")"
			}
			fmt.Printf("    %s %s=%s%s\n", prefix, change.Key, change.Proposed, current)
		}
		if len(entry.Warnings) > 0 {
			fmt.Println("  Warnings:")
			for _, warning := range entry.Warnings {
				fmt.Printf("    - %s\n", warning)
			}
		}
		fmt.Println()
	}
	if printFooter {
		fmt.Println("No files were changed.")
	}
}

func buildAMUDDiffs(plan planner.AMUDPlan) ([]diffRecord, error) {
	records := make([]diffRecord, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		originalBytes, err := os.ReadFile(entry.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.SourcePath, err)
		}
		original := string(originalBytes)
		patch, err := xmlpatch.ApplyAMUDLabels(original, entry.ProposedLabels)
		if err != nil {
			return nil, fmt.Errorf("build AMUD XML patch for %s: %w", entry.SourcePath, err)
		}
		diff := textdiff.Unified(entry.SourcePath, entry.SourcePath+" (candidate)", patch.Original, patch.Modified, 3)
		records = append(records, diffRecord{
			Container:   entry.Container,
			SourcePath:  entry.SourcePath,
			Changed:     patch.Changed,
			Operations:  patch.Ops,
			UnifiedDiff: diff,
		})
	}
	return records, nil
}

func buildTZDiffs(plan planner.TZPlan) ([]diffRecord, error) {
	records := make([]diffRecord, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		originalBytes, err := os.ReadFile(entry.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.SourcePath, err)
		}
		original := string(originalBytes)
		patch, err := xmlpatch.ApplyVariable(original, "TZ", entry.ProposedValue, "TZ")
		if err != nil {
			return nil, fmt.Errorf("build TZ XML patch for %s: %w", entry.SourcePath, err)
		}
		diff := textdiff.Unified(entry.SourcePath, entry.SourcePath+" (candidate)", patch.Original, patch.Modified, 3)
		records = append(records, diffRecord{
			Container:   entry.Container,
			SourcePath:  entry.SourcePath,
			Changed:     patch.Changed,
			Operations:  patch.Ops,
			UnifiedDiff: diff,
		})
	}
	return records, nil
}

func printAMUDDiffs(diffs []diffRecord) {
	fmt.Println("Candidate XML diff (read-only)")
	fmt.Println()
	if len(diffs) == 0 {
		fmt.Println("No diffs.")
		return
	}
	for _, diff := range diffs {
		fmt.Printf("## %s\n", diff.Container)
		if !diff.Changed {
			fmt.Println("No XML changes needed.")
			fmt.Println()
			continue
		}
		fmt.Print(diff.UnifiedDiff)
		if !strings.HasSuffix(diff.UnifiedDiff, "\n") {
			fmt.Println()
		}
		fmt.Println()
	}
	fmt.Println("No files were changed.")
}

func printTZPlan(plan planner.TZPlan, printFooter bool) {
	fmt.Println("TZ plan (read-only)")
	fmt.Printf("Plan hash: %s\n", plan.PlanHash)
	fmt.Printf("Timezone:  %s\n", plan.Timezone)
	fmt.Printf("Entries:   %d\n\n", len(plan.Entries))
	if len(plan.Entries) == 0 {
		fmt.Println("No TZ changes needed.")
		return
	}
	for _, entry := range plan.Entries {
		current := "-"
		if entry.CurrentValue != nil {
			current = *entry.CurrentValue
		}
		fmt.Printf("- %s: %s TZ %s -> %s\n", entry.Container, entry.Action, current, entry.ProposedValue)
	}
	if printFooter {
		fmt.Println()
		fmt.Println("No files were changed.")
	}
}

func printTZDiffs(diffs []diffRecord) {
	fmt.Println()
	fmt.Println("Candidate TZ XML diff (read-only)")
	fmt.Println()
	if len(diffs) == 0 {
		fmt.Println("No diffs.")
		return
	}
	for _, diff := range diffs {
		fmt.Printf("## %s\n", diff.Container)
		if !diff.Changed {
			fmt.Println("No XML changes needed.")
			fmt.Println()
			continue
		}
		fmt.Print(diff.UnifiedDiff)
		if !strings.HasSuffix(diff.UnifiedDiff, "\n") {
			fmt.Println()
		}
		fmt.Println()
	}
	fmt.Println("No files were changed.")
}

func printApplyReport(report executor.AMUDApplyReport) {
	fmt.Println("AMUD apply report")
	fmt.Printf("Plan hash:  %s\n", report.PlanHash)
	fmt.Printf("Backup dir: %s\n", report.BackupDir)
	fmt.Printf("Audit dir:  %s\n", report.AuditDir)
	fmt.Println()
	for _, result := range report.Results {
		status := "unchanged"
		if result.Changed {
			status = "changed"
		}
		fmt.Printf("- %s: %s\n", result.Container, status)
		if result.BackupPath != "" {
			fmt.Printf("  Backup: %s\n", result.BackupPath)
		}
		if result.ModifiedSHA256 != "" {
			fmt.Printf("  SHA256: %s -> %s\n", result.OriginalSHA256, result.ModifiedSHA256)
		}
	}
}

func printTZApplyReport(report executor.TZApplyReport) {
	fmt.Println("TZ apply report")
	fmt.Printf("Plan hash:  %s\n", report.PlanHash)
	fmt.Printf("Backup dir: %s\n", report.BackupDir)
	fmt.Printf("Audit dir:  %s\n", report.AuditDir)
	fmt.Println()
	for _, result := range report.Results {
		status := "unchanged"
		if result.Changed {
			status = "changed"
		}
		fmt.Printf("- %s: %s\n", result.Container, status)
		if result.BackupPath != "" {
			fmt.Printf("  Backup: %s\n", result.BackupPath)
		}
		if result.ModifiedSHA256 != "" {
			fmt.Printf("  SHA256: %s -> %s\n", result.OriginalSHA256, result.ModifiedSHA256)
		}
	}
}

func printRecreateApplyReport(report executor.RecreateApplyReport) {
	fmt.Println("Docker recreate apply report")
	fmt.Printf("Plan hash:      %s\n", report.PlanHash)
	fmt.Printf("Audit dir:      %s\n", report.AuditDir)
	fmt.Printf("Rebuild script: %s\n", report.RebuildScript)
	fmt.Printf("OK:             %t\n", report.OK)
	if report.FailureCount > 0 {
		fmt.Printf("Failures:       %d\n", report.FailureCount)
	}
	fmt.Println()
	for _, result := range report.Results {
		status := "rebuilt"
		if result.Error != "" {
			status = "failed"
		}
		fmt.Printf("- %s: %s\n", result.Container, status)
		if result.StateBefore != "" || result.StateAfter != "" {
			fmt.Printf("  State: %s -> %s\n", valueOrDash(result.StateBefore), valueOrDash(result.StateAfter))
		}
		if result.WasRunning && result.StartedAfter {
			fmt.Println("  Started again because it was running before recreate.")
		}
		if len(result.RuntimeAMUDLabels) > 0 {
			fmt.Printf("  AMUD: %s\n", amudLabelSummary(result.RuntimeAMUDLabels))
		}
		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
	}
}

func printRestoreReport(report executor.RestoreXMLReport) {
	fmt.Println("XML restore report")
	fmt.Printf("Backup:              %s\n", report.BackupPath)
	fmt.Printf("Target:              %s\n", report.TargetPath)
	fmt.Printf("Pre-restore backup:  %s\n", report.PreRestoreBackup)
	fmt.Printf("Audit dir:           %s\n", report.AuditDir)
	fmt.Printf("Backup SHA256:       %s\n", report.BackupSHA256)
	fmt.Printf("Target before SHA256:%s\n", report.TargetBeforeSHA256)
	fmt.Printf("Target after SHA256: %s\n", report.TargetAfterSHA256)
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  unraid-ai-manager inventory --templates PATH [--json]")
	fmt.Println("  unraid-ai-manager inspect-json --inspect inspect.json [--json]")
	fmt.Println("  unraid-ai-manager inspect-docker --docker-socket /var/run/docker.sock [--json]")
	fmt.Println("  unraid-ai-manager compare-runtime --templates PATH (--inspect inspect.json | --docker-socket /var/run/docker.sock | --docker-host URL) [--json]")
	fmt.Println("  unraid-ai-manager plan-recreate --templates PATH (--inspect inspect.json | --docker-socket /var/run/docker.sock | --docker-host URL) [--container NAME] [--all] [--json] [--out plan.json]")
	fmt.Println("  unraid-ai-manager plan-amud --templates PATH --local-host IP [--url-mode local|cloudflare|hybrid] [--cloudflare-domain DOMAIN] [--route NAME=SUBDOMAIN] [--container NAME] [--exclude NAME] [--include-port-only] [--runtime-filter templates|existing|running] [--inspect inspect.json | --docker-socket /var/run/docker.sock | --docker-host URL] [--diff] [--out plan.json]")
	fmt.Println("  unraid-ai-manager plan-tz --templates PATH [--tz Europe/Prague] [--container NAME] [--diff] [--out plan.json]")
	fmt.Println("  unraid-ai-manager approve-plan --plan plan.json --approvals-dir PATH [--purpose amud] [--ttl 15m] [--json]")
	fmt.Println("  unraid-ai-manager apply-amud-plan --plan plan.json --confirm-plan-hash HASH --backup-dir PATH --audit-dir PATH [--json]")
	fmt.Println("  unraid-ai-manager apply-tz-plan --plan plan.json --confirm-plan-hash HASH --backup-dir PATH --audit-dir PATH [--json]")
	fmt.Println("  unraid-ai-manager apply-recreate-plan --plan plan.json --confirm-plan-hash HASH --audit-dir PATH (--docker-socket /var/run/docker.sock | --docker-host URL) [--json]")
	fmt.Println("  unraid-ai-manager restore-xml-backup --backup backup.xml --target template.xml --confirm-backup-sha256 HASH --pre-restore-backup-dir PATH --audit-dir PATH [--json]")
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeJSONFile(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write plan JSON %s: %w", path, err)
	}
	return nil
}

type routeFlags struct {
	Map map[string]string
}

func (r *routeFlags) String() string {
	return fmt.Sprint(r.Map)
}

func (r *routeFlags) Set(value string) error {
	if r.Map == nil {
		r.Map = map[string]string{}
	}
	key, route, ok := strings.Cut(value, "=")
	key = strings.TrimSpace(key)
	route = strings.TrimSpace(route)
	if !ok || key == "" || route == "" {
		return fmt.Errorf("invalid --route value, expected NAME=SUBDOMAIN_OR_URL: %s", value)
	}
	r.Map[key] = route
	r.Map[strings.ToLower(key)] = route
	return nil
}

type nameFlags struct {
	Map map[string]bool
}

func (n *nameFlags) String() string {
	return fmt.Sprint(n.Map)
}

func (n *nameFlags) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("container name cannot be empty")
	}
	if n.Map == nil {
		n.Map = map[string]bool{}
	}
	n.Map[strings.ToLower(value)] = true
	return nil
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func portsSummary(ports []dockerxml.ConfigEntry) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, fmt.Sprintf("%s->%s/%s", port.Value, port.Target, port.Mode))
	}
	return strings.Join(parts, ", ")
}

func runtimePortsSummary(ports []dockerinspect.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		host := port.HostPort
		if port.HostIP != "" {
			host = port.HostIP + ":" + host
		}
		if host == "" {
			host = "unpublished"
		}
		parts = append(parts, fmt.Sprintf("%s->%s/%s", host, port.ContainerPort, port.Protocol))
	}
	return strings.Join(parts, ", ")
}

func amudLabelSummary(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}
	parts := []string{}
	for _, key := range []string{"amud.enable", "amud.url", "amud.name", "amud.icon"} {
		if value, ok := labels[key]; ok {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func riskSummary(findings []risk.Finding) string {
	if len(findings) == 0 {
		return "none"
	}
	counts := map[string]int{}
	order := []string{"high", "review", "info"}
	for _, finding := range findings {
		counts[finding.Severity]++
	}
	parts := []string{}
	for _, severity := range order {
		if count := counts[severity]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", severity, count))
		}
	}
	return strings.Join(parts, ", ")
}
