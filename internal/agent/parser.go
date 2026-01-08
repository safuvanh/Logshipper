package agent

import (
	"strings"
	"time"
)

// ParsedLog is the structured form sent to Elasticsearch
type ParsedLog struct {
	Timestamp string `json:"@timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`

	Namespace string `json:"namespace"`
	App       string `json:"app"`
	Pod       string `json:"pod"`
	Cluster   string `json:"cluster"`
}

// ParseLine parses a single log line for Elasticsearch
func ParseLine(line, ns, app, pod, cluster string) *ParsedLog {
	level := detectLevel(line)

	return &ParsedLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   strings.TrimSpace(line),

		Namespace: ns,
		App:       app,
		Pod:       pod,
		Cluster:   cluster,
	}
}


func detectLevel(line string) string {
	l := strings.ToLower(line)

	switch {
	case strings.Contains(l, "error"):
		return "ERROR"
	case strings.Contains(l, "warn"):
		return "WARN"
	case strings.Contains(l, "debug"):
		return "DEBUG"
	case strings.Contains(l, "info"):
		return "INFO"
	default:
		return "UNKNOWN"
	}
}
