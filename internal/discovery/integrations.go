package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"unraid-ai-manager/internal/dockerxml"
	"unraid-ai-manager/internal/planner"
)

type Options struct {
	Names map[string]bool
}

type Report struct {
	Kind    string   `json:"kind"`
	Records []Record `json:"records"`
}

type Record struct {
	Container       string   `json:"container"`
	ServiceType     string   `json:"service_type,omitempty"`
	DisplayName     string   `json:"display_name"`
	ConfigRoot      string   `json:"config_root,omitempty"`
	ConfigFiles     []string `json:"config_files,omitempty"`
	Secrets         []Secret `json:"secrets,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	DiscoveryStatus string   `json:"discovery_status"`
}

type Secret struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	SourcePath string `json:"source_path"`
	Selector   string `json:"selector"`
	Found      bool   `json:"found"`
	Preview    string `json:"preview,omitempty"`
	Length     int    `json:"length,omitempty"`
}

func DiscoverIntegrations(templates []dockerxml.Template, options Options) Report {
	report := Report{Kind: "integration-discovery"}
	for _, template := range templates {
		if len(options.Names) > 0 && !options.Names[strings.ToLower(template.Name)] {
			continue
		}
		service := planner.InferDashboardService(template.Name, template.Repository, template.TemplateURL)
		record := Record{
			Container:   template.Name,
			ServiceType: service.IntegrationType,
			DisplayName: service.DisplayName,
			ConfigRoot:  configRoot(template),
		}
		discoverKnownSecrets(&record)
		if record.ServiceType == "" {
			record.DiscoveryStatus = "generic-web"
			record.Warnings = append(record.Warnings, "No known integration signature yet; only generic dashboard metadata can be inferred.")
		} else if len(record.Secrets) == 0 {
			record.DiscoveryStatus = "known-service-no-secret-probe"
		} else {
			record.DiscoveryStatus = "known-service"
		}
		report.Records = append(report.Records, record)
	}
	return report
}

func configRoot(template dockerxml.Template) string {
	for _, path := range template.Paths() {
		target := strings.TrimSpace(path.Target)
		if target == "/config" || target == "config" {
			if strings.TrimSpace(path.Value) != "" {
				return strings.TrimSpace(path.Value)
			}
			return strings.TrimSpace(path.Default)
		}
	}
	for _, path := range template.Paths() {
		target := strings.ToLower(strings.TrimSpace(path.Target))
		if strings.Contains(target, "config") {
			if strings.TrimSpace(path.Value) != "" {
				return strings.TrimSpace(path.Value)
			}
			return strings.TrimSpace(path.Default)
		}
	}
	return ""
}

func discoverKnownSecrets(record *Record) {
	if record.ConfigRoot == "" {
		if record.ServiceType != "" {
			record.Warnings = append(record.Warnings, "No DockerMan /config path found; cannot inspect appdata config.")
		}
		return
	}
	switch record.ServiceType {
	case "radarr", "sonarr", "prowlarr", "lidarr", "readarr", "whisparr":
		discoverXMLAPIKey(record, filepath.Join(record.ConfigRoot, "config.xml"), "ApiKey")
	case "tautulli":
		discoverINIKey(record, filepath.Join(record.ConfigRoot, "config.ini"), "api_key")
	case "plex":
		discoverPlexToken(record, filepath.Join(record.ConfigRoot, "Library", "Application Support", "Plex Media Server", "Preferences.xml"))
	case "cloudflare_tunnel":
		discoverCloudflareTunnelFiles(record)
	}
}

func discoverXMLAPIKey(record *Record, path string, element string) {
	record.ConfigFiles = append(record.ConfigFiles, path)
	payload, ok := readSmallFile(record, path)
	secret := Secret{
		Name:       "api_key",
		Kind:       "xml-element",
		SourcePath: path,
		Selector:   element,
	}
	if ok {
		value := xmlElementValue(payload, element)
		secret.Found = value != ""
		secret.Preview = maskSecret(value)
		secret.Length = len(value)
	}
	record.Secrets = append(record.Secrets, secret)
}

func discoverINIKey(record *Record, path string, key string) {
	record.ConfigFiles = append(record.ConfigFiles, path)
	payload, ok := readSmallFile(record, path)
	secret := Secret{
		Name:       "api_key",
		Kind:       "ini-key",
		SourcePath: path,
		Selector:   key,
	}
	if ok {
		value := iniValue(payload, key)
		secret.Found = value != ""
		secret.Preview = maskSecret(value)
		secret.Length = len(value)
	}
	record.Secrets = append(record.Secrets, secret)
}

func discoverPlexToken(record *Record, path string) {
	record.ConfigFiles = append(record.ConfigFiles, path)
	payload, ok := readSmallFile(record, path)
	secret := Secret{
		Name:       "plex_token",
		Kind:       "xml-attribute",
		SourcePath: path,
		Selector:   "PlexOnlineToken",
	}
	if ok {
		value := xmlAttributeValue(payload, "PlexOnlineToken")
		secret.Found = value != ""
		secret.Preview = maskSecret(value)
		secret.Length = len(value)
	}
	record.Secrets = append(record.Secrets, secret)
}

func discoverCloudflareTunnelFiles(record *Record) {
	candidates := []string{
		filepath.Join(record.ConfigRoot, "config.yml"),
		filepath.Join(record.ConfigRoot, "config.yaml"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			record.ConfigFiles = append(record.ConfigFiles, path)
		}
	}
	if len(record.ConfigFiles) == 0 {
		record.Warnings = append(record.Warnings, "No Cloudflare Tunnel config.yml/config.yaml found in config root.")
	}
}

func readSmallFile(record *Record, path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		record.Warnings = append(record.Warnings, fmt.Sprintf("Config file not found: %s", path))
		return "", false
	}
	if info.IsDir() {
		record.Warnings = append(record.Warnings, fmt.Sprintf("Config path is a directory: %s", path))
		return "", false
	}
	if info.Size() > 2<<20 {
		record.Warnings = append(record.Warnings, fmt.Sprintf("Config file is larger than 2 MiB and was not read: %s", path))
		return "", false
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		record.Warnings = append(record.Warnings, fmt.Sprintf("Cannot read config file %s: %v", path, err))
		return "", false
	}
	return string(payload), true
}

func xmlElementValue(payload string, element string) string {
	pattern := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(element) + `>\s*([^<\s]+)\s*</` + regexp.QuoteMeta(element) + `>`)
	match := pattern.FindStringSubmatch(payload)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func xmlAttributeValue(payload string, attribute string) string {
	pattern := regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(attribute) + `\s*=\s*"([^"]+)"`)
	match := pattern.FindStringSubmatch(payload)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func iniValue(payload string, key string) string {
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		foundKey, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(foundKey), key) {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 10 {
		return "********"
	}
	return value[:6] + "..." + value[len(value)-6:]
}
