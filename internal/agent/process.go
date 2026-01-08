package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (a *Agent) processLine(appName, pod, ns, line string, state *followState) {

	// Check if namespace matches (supports multiple namespaces and wildcard)
	if !a.matchesNamespace(ns) {
		return
	}

	// Extract the log date from the Kubernetes timestamp prefix (YYYY-MM-DD format)
	// Expected log format: 2026-01-05T23:59:41.890737119Z stdout F [message]
	// NOTE: Kubernetes logs are ALWAYS in UTC, regardless of pod's local timezone
	// Multi-line logs (continuation lines) may not have timestamps - we track the last valid one
	var logDate string
	var logTimestamp string
	var isContinuationLine bool
	
	if len(line) >= 10 {
		logDate = line[:10] // Extract YYYY-MM-DD
		// Validate it's a valid date format
		if _, err := time.Parse("2006-01-02", logDate); err != nil {
			// ✅ NOT a valid date - this is a continuation line (e.g., stack trace)
			// Use the last valid timestamp from the previous line in the same log entry
			isContinuationLine = true
			logTimestamp = state.lastValidTimestamp
		} else {
			// ✅ Valid timestamp line - extract and store it
			if len(line) >= 30 && line[len(line)-1] != 'Z' {
				// Try to find the 'Z' that marks end of RFC3339 timestamp
				for i := 10; i < 40 && i < len(line); i++ {
					if line[i] == 'Z' {
						logTimestamp = line[:i+1] // "2026-01-05T10:30:00.123456789Z"
						state.lastValidTimestamp = logTimestamp
						break
					}
				}
			}
			
			// Fallback if we couldn't parse full timestamp
			if logTimestamp == "" {
				logTimestamp = logDate + "T00:00:00Z"
				state.lastValidTimestamp = logTimestamp
			}
		}
	} else {
		// Log line too short to contain valid timestamp - treat as continuation line
		isContinuationLine = true
		logTimestamp = state.lastValidTimestamp
	}

	// For S3: Accept logs from today and yesterday (handles day boundary race conditions)
	// CRITICAL: Use UTC, not pod's local timezone (which might be IST, EST, etc.)
	// This ensures date comparison matches the UTC timestamps in Kubernetes logs
	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Check if log date is recent enough for S3 upload
	// For continuation lines, use the extracted timestamp's date
	isRecentLog := false
	if isContinuationLine && logTimestamp != "" && len(logTimestamp) >= 10 {
		// Extract date from the timestamp (e.g., "2026-01-05" from "2026-01-05T10:30:00Z")
		tsDate := logTimestamp[:10]
		isRecentLog = tsDate == today || tsDate == yesterday
	} else if !isContinuationLine {
		isRecentLog = logDate == today || logDate == yesterday
	}

	// Early exit ONLY if: S3 is the ONLY output enabled AND log is too old
	// This allows ES to still process all logs even when S3 is enabled
	if a.cfg.S3Enabled && !a.cfg.ESEnabled && !isRecentLog {
		return
	}

	// ---------- S3 PATH: Recent logs only (today and yesterday) ----------
	if a.cfg.S3Enabled && isRecentLog {
		dir := filepath.Join(a.cfg.TmpDir, ns, appName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir tmp dir err: %v\n", err)
			return
		}

		// Use the log's actual date (logDate for normal lines, or extracted from timestamp for continuation lines)
		fileDate := logDate
		if isContinuationLine && logTimestamp != "" && len(logTimestamp) >= 10 {
			fileDate = logTimestamp[:10]
		}
		tmpFile := filepath.Join(dir, fmt.Sprintf("%s-%s.log", pod, fileDate))
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

		// logTimestamp already extracted and set above
		// ✅ Uses actual log timestamp for normal lines
		// ✅ Uses last valid timestamp for continuation lines (stack traces, exceptions)
		if logTimestamp == "" {
			// Last resort fallback - should rarely happen
			logTimestamp = time.Now().UTC().Format(time.RFC3339Nano)
		}

		doc := map[string]interface{}{
			"@timestamp": logTimestamp,  // ✅ ACTUAL log timestamp, not processing time
			"namespace":  ns,
			"app":        appName,
			"pod":        pod,
			"message":    line,
		}

		a.esClient.Add(doc)
		// Uncomment below for very verbose logging (logs every document):
		// fmt.Printf("[ES-TRACE] Added log: ns=%s app=%s pod=%s ts=%s\n", ns, appName, pod, logTimestamp)
	}
}

