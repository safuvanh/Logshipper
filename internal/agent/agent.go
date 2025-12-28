package agent

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"logshipper/internal/config"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Agent struct {
	cfg *config.Config

	s3client *s3.Client

	mu sync.Mutex
	// map resolved target path -> follower
	followers map[string]*followState

	checksums map[string]string

	// watchdog state
	lastActivity map[string]time.Time // key: ns/app/pod
	warned       map[string]bool      // whether we've warned for the key
}

func New(cfg *config.Config) (*Agent, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	s3c := s3.NewFromConfig(awsCfg)

	_ = os.MkdirAll(cfg.TmpDir, 0o755)

	return &Agent{
		cfg:          cfg,
		s3client:     s3c,
		followers:    make(map[string]*followState),
		checksums:    make(map[string]string),
		lastActivity: make(map[string]time.Time),
		warned:       make(map[string]bool),
	}, nil
}

func (a *Agent) Run() {
	if err := a.scanAndStartFollowers(); err != nil {
		log.Printf("initial scan err (may be fine): %v", err)
	}
	go a.watchContainersDir()
	go a.watchPodsDir()

	// start the quiet watchdog only if enabled in config
	if a.cfg != nil && a.cfg.WatchdogEnabled {
		log.Printf("watchdog enabled (threshold %d min)", a.cfg.QuietWarnMin)
		go a.startWatchdog()
	} else {
		log.Printf("watchdog disabled by configuration (WATCHDOG_ENABLED=false)")
	}

	// blocking upload loop
	a.uploadLoop()
}

// matchesNamespace returns true if the provided namespace should be processed.
// It supports:
//  - explicit config.TargetNSList (comma-separated list populated at startup)
//  - fallback to single config.TargetNS for backward compatibility
// If both are populated, TargetNSList takes precedence.
func (a *Agent) matchesNamespace(ns string) bool {
	if a == nil || a.cfg == nil {
		return false
	}

	// prefer TargetNSList (new)
	if len(a.cfg.TargetNSList) > 0 {
		for _, candidate := range a.cfg.TargetNSList {
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			if candidate == "*" || strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(ns)) {
				return true
			}
		}
		return false
	}

	// fallback to single TargetNS (backwards compatible)
	if a.cfg.TargetNS == "*" || strings.EqualFold(strings.TrimSpace(a.cfg.TargetNS), strings.TrimSpace(ns)) {
		return true
	}
	return false
}
