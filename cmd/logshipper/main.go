package main

import (
	"log"

	"logshipper/internal/agent"
	"logshipper/internal/config"
	"logshipper/internal/health"
	"logshipper/internal/es"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := config.LoadFromEnv()

	// Validate S3 only if enabled
	if cfg.S3Enabled && cfg.S3Bucket == "" {
		log.Fatalf("S3_BUCKET is required when ENABLE_S3=true")
	}

	// Validate Elasticsearch only if enabled
	if cfg.ESEnabled {
		if cfg.ESURL == "" {
			log.Fatalf("ES_URL is required when ENABLE_ES=true")
		}
		// ES_USERNAME and ES_PASSWORD are optional (only needed if auth is required)
	}

	// Banner + health
	health.PrintBanner(cfg.AppName, cfg.AppVersion, cfg.TargetNS)
	go health.Start(":8080")

	// Init Elasticsearch metrics BEFORE agent (if enabled)
	if cfg.ESEnabled {
		es.InitMetrics()
		go es.StartMetricsServer(":8080") // /metrics
	}

	// Init agent (creates ES client if ESEnabled=true and starts worker)
	ag, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	// Log ES status after agent init
	if cfg.ESEnabled {
		authStatus := "❌ NO AUTH"
		if cfg.ESUsername != "" && cfg.ESPassword != "" {
			authStatus = "✅ WITH AUTH (username/password)"
		} else if cfg.ESUsername != "" || cfg.ESPassword != "" {
			authStatus = "⚠️  PARTIAL AUTH (only username or password set)"
		}
		
		log.Printf("[CONFIG] ✅ Elasticsearch ENABLED:")
		log.Printf("         URL: %s", cfg.ESURL)
		log.Printf("         Authentication: %s", authStatus)
		log.Printf("         Index Prefix: %s", cfg.ESIndexPrefix)
		log.Printf("         Batch Size: %d documents", cfg.ESBatchSize)
		log.Printf("         Flush Interval: %d seconds", cfg.ESFlushInterval)
		log.Printf("         Max Buffer: %d documents", cfg.ESMaxBuffer)
		log.Printf("         Metrics Server: http://localhost:8080/metrics")
	} else {
		log.Printf("[CONFIG] ⚠️  Elasticsearch DISABLED (ENABLE_ES not set)")
	}

	if cfg.S3Enabled {
		log.Printf("[CONFIG] ✅ S3 ENABLED - Bucket: %s", cfg.S3Bucket)
	} else {
		log.Printf("[CONFIG] ⚠️  S3 DISABLED (ENABLE_S3 not set)")
	}

	log.Printf("[CONFIG] Target Namespaces: %v", cfg.TargetNSList)
	log.Printf("[START] Application starting... Ready to process logs")
	log.Printf("==========================================================================")

	// Run agent (blocking)
	ag.Run()
}
