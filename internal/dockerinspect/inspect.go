package dockerinspect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Container struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	RuntimeImage  string            `json:"runtime_image"`
	State         string            `json:"state"`
	Status        string            `json:"status"`
	NetworkMode   string            `json:"network_mode"`
	Labels        map[string]string `json:"labels"`
	Env           map[string]string `json:"env"`
	Ports         []Port            `json:"ports"`
	RawSourcePath string            `json:"raw_source_path,omitempty"`
}

type Port struct {
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip,omitempty"`
	HostPort      string `json:"host_port,omitempty"`
}

type rawContainer struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Image  string `json:"Image"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
		Env    []string          `json:"Env"`
	} `json:"Config"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
	HostConfig struct {
		NetworkMode  string                   `json:"NetworkMode"`
		PortBindings map[string][]portBinding `json:"PortBindings"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports map[string][]portBinding `json:"Ports"`
	} `json:"NetworkSettings"`
}

type portBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

func LoadPath(path string) ([]Container, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return LoadFile(path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var jsonPaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			jsonPaths = append(jsonPaths, filepath.Join(path, entry.Name()))
		}
	}
	sort.Strings(jsonPaths)

	var containers []Container
	for _, jsonPath := range jsonPaths {
		loaded, err := LoadFile(jsonPath)
		if err != nil {
			return nil, err
		}
		containers = append(containers, loaded...)
	}
	return containers, nil
}

func LoadFile(path string) ([]Container, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseBytes(payload, path)
}

func ParseBytes(payload []byte, source string) ([]Container, error) {
	var rawList []rawContainer
	if err := json.Unmarshal(payload, &rawList); err != nil {
		var raw rawContainer
		if errObject := json.Unmarshal(payload, &raw); errObject != nil {
			return nil, fmt.Errorf("parse docker inspect JSON %s: %w", source, err)
		}
		rawList = []rawContainer{raw}
	}

	containers := make([]Container, 0, len(rawList))
	for _, raw := range rawList {
		container := normalize(raw)
		container.RawSourcePath = source
		containers = append(containers, container)
	}
	return containers, nil
}

func IndexByName(containers []Container) map[string]Container {
	index := map[string]Container{}
	for _, container := range containers {
		if container.Name == "" {
			continue
		}
		index[container.Name] = container
		index[strings.ToLower(container.Name)] = container
	}
	return index
}

func normalize(raw rawContainer) Container {
	name := strings.TrimPrefix(strings.TrimSpace(raw.Name), "/")
	labels := raw.Config.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	return Container{
		ID:           raw.ID,
		Name:         name,
		Image:        raw.Config.Image,
		RuntimeImage: raw.Image,
		State:        raw.State.Status,
		Status:       raw.State.Status,
		NetworkMode:  raw.HostConfig.NetworkMode,
		Labels:       labels,
		Env:          parseEnv(raw.Config.Env),
		Ports:        normalizePorts(raw),
	}
}

func parseEnv(values []string) map[string]string {
	env := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok {
			env[value] = ""
			continue
		}
		env[key] = val
	}
	return env
}

func normalizePorts(raw rawContainer) []Port {
	source := raw.NetworkSettings.Ports
	if len(source) == 0 {
		source = raw.HostConfig.PortBindings
	}

	var keys []string
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var ports []Port
	for _, key := range keys {
		containerPort, protocol := splitDockerPortKey(key)
		bindings := source[key]
		if len(bindings) == 0 {
			ports = append(ports, Port{
				ContainerPort: containerPort,
				Protocol:      protocol,
			})
			continue
		}
		for _, binding := range bindings {
			ports = append(ports, Port{
				ContainerPort: containerPort,
				Protocol:      protocol,
				HostIP:        binding.HostIP,
				HostPort:      binding.HostPort,
			})
		}
	}
	return ports
}

func splitDockerPortKey(key string) (string, string) {
	port, protocol, ok := strings.Cut(key, "/")
	if !ok {
		return key, ""
	}
	return port, protocol
}
