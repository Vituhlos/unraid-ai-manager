package approval

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Record struct {
	PlanHash  string `json:"plan_hash"`
	TokenHash string `json:"token_hash"`
	Purpose   string `json:"purpose"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	UsedAt    string `json:"used_at,omitempty"`
}

type GrantResult struct {
	PlanHash  string `json:"plan_hash"`
	Token     string `json:"token"`
	Purpose   string `json:"purpose"`
	ExpiresAt string `json:"expires_at"`
	Path      string `json:"path"`
}

func Grant(dir string, planHash string, purpose string, ttl time.Duration, now time.Time) (GrantResult, error) {
	if dir == "" {
		return GrantResult{}, errors.New("approval dir is required")
	}
	if planHash == "" {
		return GrantResult{}, errors.New("plan hash is required")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return GrantResult{}, err
	}

	token, err := randomToken()
	if err != nil {
		return GrantResult{}, err
	}
	record := Record{
		PlanHash:  planHash,
		TokenHash: tokenHash(token),
		Purpose:   purpose,
		CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: now.Add(ttl).Format(time.RFC3339),
	}
	path := filepath.Join(dir, safeName(now.Format("20060102T150405Z")+"_"+purpose+"_"+planHash+".json"))
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return GrantResult{}, err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return GrantResult{}, err
	}
	return GrantResult{
		PlanHash:  planHash,
		Token:     token,
		Purpose:   purpose,
		ExpiresAt: record.ExpiresAt,
		Path:      path,
	}, nil
}

func Consume(dir string, planHash string, token string, now time.Time) error {
	if dir == "" {
		return errors.New("approval dir is required")
	}
	if planHash == "" {
		return errors.New("plan hash is required")
	}
	if token == "" {
		return errors.New("approval token is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	records, err := findRecords(dir, planHash)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("no approval record found for plan hash %s", planHash)
	}

	wantedHash := tokenHash(token)
	for _, path := range records {
		record, err := readRecord(path)
		if err != nil {
			return err
		}
		if record.TokenHash != wantedHash {
			continue
		}
		if record.UsedAt != "" {
			return errors.New("approval token was already used")
		}
		expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt)
		if err != nil {
			return fmt.Errorf("approval record has invalid expiration: %w", err)
		}
		if now.After(expiresAt) {
			return errors.New("approval token expired")
		}
		record.UsedAt = now.Format(time.RFC3339)
		return writeRecord(path, record)
	}
	return errors.New("approval token does not match plan hash")
}

func ExtractPlanHashFromFile(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var generic map[string]any
	if err := json.Unmarshal(payload, &generic); err != nil {
		return "", err
	}
	if hash, ok := generic["plan_hash"].(string); ok && hash != "" {
		return hash, nil
	}
	if wrapped, ok := generic["plan"].(map[string]any); ok {
		if hash, ok := wrapped["plan_hash"].(string); ok && hash != "" {
			return hash, nil
		}
	}
	return "", errors.New("plan_hash not found in plan file")
}

func findRecords(dir string, planHash string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		record, err := readRecord(path)
		if err != nil {
			continue
		}
		if record.PlanHash == planHash {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func readRecord(path string) (Record, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(payload, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func writeRecord(path string, record Record) error {
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o600)
}

func randomToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func safeName(value string) string {
	replacer := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(value)
}
