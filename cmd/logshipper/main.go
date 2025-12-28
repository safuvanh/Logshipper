package main

import (
	"log"

	"logshipper/internal/agent"
	"logshipper/internal/config"
	"logshipper/internal/health"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	cfg := config.LoadFromEnv()
	if cfg.S3Bucket == "" {
		log.Fatalf("S3_BUCKET is required")
	}
	health.PrintBanner(cfg.AppName, cfg.AppVersion, cfg.TargetNS)
	go health.Start(":8080")
	ag, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}
	ag.Run()
}
