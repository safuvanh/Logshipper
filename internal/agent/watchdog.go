package agent

import (
    "log"
    "time"
)

func (a *Agent) startWatchdog() {
    threshold := time.Duration(a.cfg.QuietWarnMin) * time.Minute
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        now := time.Now()

        a.mu.Lock()
        for key, last := range a.lastActivity {

            // skip if we've already warned for this quiet period
            if a.warned[key] {
                continue
            }

            quietFor := now.Sub(last)

            if quietFor > threshold {
                log.Printf(
                    "WARN: No log activity for [%s] for %d minutes (threshold=%d min)",
                    key,
                    int(quietFor.Minutes()),
                    a.cfg.QuietWarnMin,
                )
                a.warned[key] = true
            }
        }
        a.mu.Unlock()
    }
}
