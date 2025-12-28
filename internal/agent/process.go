package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (a *Agent) processLine(appName, pod, ns, line string) {

	// Check if namespace matches (supports multiple namespaces and wildcard)
	if !a.matchesNamespace(ns) {
		return
	}

	today := time.Now().Format("2006-01-02")
	
	// Check if log has today's date (required for S3 file organization)
	isTodayLog := len(line) >= len(today) && line[:len(today)] == today
	
	// Early exit ONLY if: S3 is the ONLY output enabled AND log is not today's
	// This allows ES to still process old logs even when S3 is enabled
	if a.cfg.S3Enabled && !a.cfg.ESEnabled && !isTodayLog {
		return
	}

	// ---------- S3 PATH: Only today's logs ----------
	if a.cfg.S3Enabled && isTodayLog {
		dir := filepath.Join(a.cfg.TmpDir, ns, appName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir tmp dir err: %v\n", err)
			return
		}

		tmpFile := filepath.Join(dir, fmt.Sprintf("%s-%s.log", pod, today))
		f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open tmp file err: %v\n", err)
			return
		}
		_, _ = f.WriteString(line)
		_ = f.Close()
	}

	// ---------- ES PATH ----------
	if a.cfg.ESEnabled {
		if a.esClient == nil {
			fmt.Fprintf(os.Stderr, "[ES-ERROR] esClient is nil - ES not initialized!\n")
			return
		}

		doc := map[string]interface{}{
			"@timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"namespace":  ns,
			"app":        appName,
			"pod":        pod,
			"message":    line,
		}

		a.esClient.Add(doc)
		// Uncomment below for very verbose logging (logs every document):
		// fmt.Printf("[ES-TRACE] Added log: ns=%s app=%s pod=%s\n", ns, appName, pod)
	}
}

