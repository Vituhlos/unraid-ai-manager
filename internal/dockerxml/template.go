package dockerxml

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ConfigEntry struct {
	Name        string `xml:"Name,attr" json:"name"`
	Target      string `xml:"Target,attr" json:"target"`
	Default     string `xml:"Default,attr" json:"default"`
	Mode        string `xml:"Mode,attr" json:"mode"`
	Description string `xml:"Description,attr" json:"description"`
	Type        string `xml:"Type,attr" json:"type"`
	Display     string `xml:"Display,attr" json:"display"`
	Required    string `xml:"Required,attr" json:"required"`
	Mask        string `xml:"Mask,attr" json:"mask"`
	Value       string `xml:",chardata" json:"value"`
}

type Template struct {
	XMLName       xml.Name      `xml:"Container" json:"-"`
	SourcePath    string        `xml:"-" json:"source_path"`
	Version       string        `xml:"version,attr" json:"version"`
	Name          string        `xml:"Name" json:"name"`
	Repository    string        `xml:"Repository" json:"repository"`
	Registry      string        `xml:"Registry" json:"registry"`
	Network       string        `xml:"Network" json:"network"`
	MyIP          string        `xml:"MyIP" json:"my_ip"`
	Shell         string        `xml:"Shell" json:"shell"`
	PrivilegedRaw string        `xml:"Privileged" json:"-"`
	Privileged    bool          `xml:"-" json:"privileged"`
	Support       string        `xml:"Support" json:"support"`
	Project       string        `xml:"Project" json:"project"`
	ReadMe        string        `xml:"ReadMe" json:"readme"`
	Overview      string        `xml:"Overview" json:"overview"`
	Category      string        `xml:"Category" json:"category"`
	WebUI         string        `xml:"WebUI" json:"web_ui"`
	TemplateURL   string        `xml:"TemplateURL" json:"template_url"`
	Icon          string        `xml:"Icon" json:"icon"`
	ExtraParams   string        `xml:"ExtraParams" json:"extra_params"`
	PostArgs      string        `xml:"PostArgs" json:"post_args"`
	CPUset        string        `xml:"CPUset" json:"cpu_set"`
	DateInstalled string        `xml:"DateInstalled" json:"date_installed"`
	Configs       []ConfigEntry `xml:"Config" json:"configs"`
}

func LoadTemplates(path string) ([]Template, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		template, err := ParseTemplateFile(path)
		if err != nil {
			return nil, err
		}
		return []Template{template}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var xmlPaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".xml") {
			xmlPaths = append(xmlPaths, filepath.Join(path, entry.Name()))
		}
	}
	sort.Strings(xmlPaths)

	templates := make([]Template, 0, len(xmlPaths))
	for _, xmlPath := range xmlPaths {
		template, err := ParseTemplateFile(xmlPath)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func ParseTemplateFile(path string) (Template, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Template{}, err
	}

	var template Template
	if err := xml.Unmarshal(payload, &template); err != nil {
		return Template{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if template.XMLName.Local != "Container" {
		return Template{}, fmt.Errorf("%s is not a DockerMan Container template", path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	template.SourcePath = abs
	template.Privileged = strings.EqualFold(strings.TrimSpace(template.PrivilegedRaw), "true")
	template.trimStrings()
	for index := range template.Configs {
		template.Configs[index].Value = strings.TrimSpace(template.Configs[index].Value)
	}
	return template, nil
}

func (t Template) Ports() []ConfigEntry {
	return t.configsByType("port")
}

func (t Template) Paths() []ConfigEntry {
	return t.configsByType("path")
}

func (t Template) Variables() []ConfigEntry {
	return t.configsByType("variable")
}

func (t Template) Labels() []ConfigEntry {
	return t.configsByType("label")
}

func (t Template) LabelMap() map[string]string {
	labels := map[string]string{}
	for _, label := range t.Labels() {
		if label.Target != "" {
			labels[label.Target] = label.Value
		}
	}
	return labels
}

func (t Template) configsByType(configType string) []ConfigEntry {
	var configs []ConfigEntry
	for _, config := range t.Configs {
		if strings.EqualFold(config.Type, configType) {
			configs = append(configs, config)
		}
	}
	return configs
}

func (t *Template) trimStrings() {
	t.Name = strings.TrimSpace(t.Name)
	t.Repository = strings.TrimSpace(t.Repository)
	t.Registry = strings.TrimSpace(t.Registry)
	t.Network = strings.TrimSpace(t.Network)
	t.MyIP = strings.TrimSpace(t.MyIP)
	t.Shell = strings.TrimSpace(t.Shell)
	t.Support = strings.TrimSpace(t.Support)
	t.Project = strings.TrimSpace(t.Project)
	t.ReadMe = strings.TrimSpace(t.ReadMe)
	t.Overview = strings.TrimSpace(t.Overview)
	t.Category = strings.TrimSpace(t.Category)
	t.WebUI = strings.TrimSpace(t.WebUI)
	t.TemplateURL = strings.TrimSpace(t.TemplateURL)
	t.Icon = strings.TrimSpace(t.Icon)
	t.ExtraParams = strings.TrimSpace(t.ExtraParams)
	t.PostArgs = strings.TrimSpace(t.PostArgs)
	t.CPUset = strings.TrimSpace(t.CPUset)
	t.DateInstalled = strings.TrimSpace(t.DateInstalled)
}
