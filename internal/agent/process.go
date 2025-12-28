package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// processLine writes application log lines into per-pod temp files.
// It persists only lines that start with today's date (YYYY-MM-DD) to keep files per-day.
// tmp layout: <TMP_DIR>/<namespace>/<appName>/<pod>-YYYY-MM-DD.log
func (a *Agent) processLine(appName, pod, ns, line string) {
	if ns != a.cfg.TargetNS {
		return
	}
	today := time.Now().Format("2006-01-02")
	if len(line) < len(today) || line[:len(today)] != today {
		// skip lines that don't begin with today's date
		return
	}
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
