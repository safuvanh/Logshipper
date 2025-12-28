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
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Bool env reader: false only if value is "false" or "0" or "no".
func getenvBool(key string, def bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return def
	}
	if v == "false" || v == "0" || v == "no" {
		return false
	}
	return true
}

// splitTrim splits comma-separated values and trims spaces, ignoring empty items.
func splitTrim(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
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
		S3Bucket:        getenv("S3_BUCKET", "ifsc-eks-logs"),
		ClusterName:     getenv("CLUSTER_NAME", "cluster"),
		IntervalSec:     getenvInt("INTERVAL", 60),
		QuietWarnMin:    getenvInt("QUIET_WARN_MIN", 10),
		ExcludeList:     getenv("EXCLUDE_LIST", "istio-proxy,istio-init,envoy,proxy,istio"),
		TmpDir:          getenv("TMP_DIR", "/var/log/agent-out"),
		AppName:         getenv("APP_NAME", "logshipper"),
		AppVersion:      getenv("APP_VERSION", "v1.0.0"),
		CleanupDays:     getenvInt("CLEANUP_DAYS", 7),
		WatchdogEnabled: getenvBool("WATCHDOG_ENABLED", true),
	}
}
