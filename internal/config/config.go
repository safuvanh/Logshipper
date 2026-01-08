package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ContainersDir   string
	PodsDir         string
	TargetNS        string
	TargetNSList    []string
	S3Bucket        string
	ClusterName     string
	IntervalSec     int
	QuietWarnMin    int
	ExcludeList     string
	TmpDir          string
	AppName         string
	AppVersion      string
	CleanupDays     int
	WatchdogEnabled bool

	// Output switches
	S3Enabled bool
	ESEnabled bool

	// Elasticsearch config
	ESURL           string
	ESUsername      string
	ESPassword      string
	ESIndexPrefix   string
	ESFlushInterval int
	ESBatchSize     int
	ESMaxBuffer     int
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	n, err := strconv.Atoi(v)
	if err != nil || v == "" {
		return def
	}
	return n
}

func getenvBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return def
	}
	return !(v == "false" || v == "0" || v == "no")
}

func splitTrim(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func LoadFromEnv() *Config {
	rawNS := getenv("TARGET_NS", "ifsc-uat")

	return &Config{
		ContainersDir:   getenv("CONTAINERS_DIR", "/var/log/containers"),
		PodsDir:         getenv("PODS_DIR", "/var/log/pods"),
		TargetNS:        rawNS,
		TargetNSList:    splitTrim(rawNS),
		S3Bucket:        getenv("S3_BUCKET", ""),
		ClusterName:     getenv("CLUSTER_NAME", "cluster"),
		IntervalSec:     getenvInt("INTERVAL", 60),
		QuietWarnMin:    getenvInt("QUIET_WARN_MIN", 10),
		ExcludeList:     getenv("EXCLUDE_LIST", "istio-proxy,istio-init,envoy,proxy,istio"),
		TmpDir:          getenv("TMP_DIR", "/var/log/agent-out"),
		AppName:         getenv("APP_NAME", "logshipper"),
		AppVersion:      getenv("APP_VERSION", "v1.0.0"),
		CleanupDays:     getenvInt("CLEANUP_DAYS", 7),
		WatchdogEnabled: getenvBool("WATCHDOG_ENABLED", false),

		S3Enabled: getenvBool("ENABLE_S3", false),
		ESEnabled: getenvBool("ENABLE_ES", false),

		ESURL:           getenv("ES_URL", ""),
		ESUsername:      getenv("ES_USERNAME", ""),
		ESPassword:      getenv("ES_PASSWORD", ""),
		ESIndexPrefix:   getenv("ES_INDEX_PREFIX", "k8s-logs"),
		ESFlushInterval: getenvInt("ES_FLUSH_INTERVAL_SEC", 5),
		ESBatchSize:     getenvInt("ES_BATCH_SIZE", 500),
		ESMaxBuffer:     getenvInt("ES_MAX_BUFFER", 5000),
	}
}
