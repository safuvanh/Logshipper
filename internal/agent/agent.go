package agent

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"logshipper/internal/config"
	"logshipper/internal/es"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Agent struct {
	cfg *config.Config

	s3client *s3.Client
	esClient *es.Client

	mu sync.Mutex
	followers map[string]*followState
	checksums map[string]string

	// watchdog state
	lastActivity map[string]time.Time
	warned       map[string]bool
}

func New(cfg *config.Config) (*Agent, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	s3c := s3.NewFromConfig(awsCfg)

	_ = os.MkdirAll(cfg.TmpDir, 0o755)

	var esClient *es.Client
	if cfg.ESEnabled {
		esClient = es.NewClient(
			cfg.ESURL,
			cfg.ESUsername,
			cfg.ESPassword,
			cfg.ESIndexPrefix,
			cfg.ESBatchSize,
			cfg.ESFlushInterval,
			0, // MaxBuffer handled internally
		)

		esClient.StartWorker()
		log.Printf("Elasticsearch output enabled (index=%s)", cfg.ESIndexPrefix)
	} else {
		log.Printf("Elasticsearch output disabled")
	}

	return &Agent{
		cfg:          cfg,
		s3client:     s3c,
		esClient:     esClient,
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

	if a.cfg.WatchdogEnabled {
		log.Printf("watchdog enabled (threshold %d min)", a.cfg.QuietWarnMin)
		go a.startWatchdog()
	} else {
		log.Printf("watchdog disabled")
	}

	a.uploadLoop()
}

// Namespace matcher (unchanged logic)
func (a *Agent) matchesNamespace(ns string) bool {
	if a == nil || a.cfg == nil {
		return false
	}

	if len(a.cfg.TargetNSList) > 0 {
		for _, candidate := range a.cfg.TargetNSList {
			if candidate == "*" || strings.EqualFold(candidate, ns) {
				return true
			}
		}
		return false
	}

	return a.cfg.TargetNS == "*" || strings.EqualFold(a.cfg.TargetNS, ns)
}

/*
	SetESClient fixes:
	cmd/logshipper/main.go: ag.SetESClient undefined
*/
func (a *Agent) SetESClient(c *es.Client) {
	a.esClient = c
}
