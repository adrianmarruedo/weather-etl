package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Loader struct {
	rawDir       string
	processedDir string
}

func New(rawDir, processedDir string) (*Loader, error) {
	for _, dir := range []string{rawDir, processedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return &Loader{rawDir: rawDir, processedDir: processedDir}, nil
}

func (l *Loader) SaveRaw(data any) (string, error) {
	return appendToFile(l.rawDir, data)
}

func (l *Loader) SaveProcessed(data any) (string, error) {
	return appendToFile(l.processedDir, data)
}

func appendToFile(dir string, data any) (string, error) {
	filename := filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+".json")

	var records []json.RawMessage

	if existing, err := os.ReadFile(filename); err == nil && len(existing) > 0 {
		if err := json.Unmarshal(existing, &records); err != nil {
			records = []json.RawMessage{}
		}
	}

	entry, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal record: %w", err)
	}
	records = append(records, json.RawMessage(entry))

	out, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal records array: %w", err)
	}

	if err := os.WriteFile(filename, out, 0o644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filename, err)
	}
	return filename, nil
}
