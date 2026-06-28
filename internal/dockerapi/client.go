package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"unraid-ai-manager/internal/dockerinspect"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type ContainerSummary struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

func NewUnixSocketClient(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{
		baseURL: "http://docker",
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

func NewHTTPClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) InspectAll(ctx context.Context) ([]dockerinspect.Container, error) {
	summaries, err := c.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	containers := make([]dockerinspect.Container, 0, len(summaries))
	for _, summary := range summaries {
		container, err := c.InspectContainer(ctx, summary.ID)
		if err != nil {
			return nil, err
		}
		containers = append(containers, container)
	}
	return containers, nil
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	payload, err := c.get(ctx, "/containers/json?all=1")
	if err != nil {
		return nil, err
	}
	var summaries []ContainerSummary
	if err := json.Unmarshal(payload, &summaries); err != nil {
		return nil, fmt.Errorf("parse Docker container list: %w", err)
	}
	return summaries, nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (dockerinspect.Container, error) {
	payload, err := c.get(ctx, "/containers/"+id+"/json")
	if err != nil {
		return dockerinspect.Container{}, err
	}
	containers, err := dockerinspect.ParseBytes(payload, "docker-api:"+id)
	if err != nil {
		return dockerinspect.Container{}, err
	}
	if len(containers) != 1 {
		return dockerinspect.Container{}, fmt.Errorf("expected one inspect object for %s, got %d", id, len(containers))
	}
	return containers[0], nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	_, err := c.post(ctx, "/containers/"+id+"/start", nil)
	return err
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Docker API GET %s failed: HTTP %d: %s", path, response.StatusCode, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func (c *Client) post(ctx context.Context, path string, body io.Reader) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Docker API POST %s failed: HTTP %d: %s", path, response.StatusCode, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}
