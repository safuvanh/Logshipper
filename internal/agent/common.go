package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// followState is the per-follower control struct used across files.
type followState struct {
	cancel chan struct{}
	done   chan struct{}
}

// fileID represents device+inode of a file.
type fileID struct {
	Dev uint64
	Ino uint64
}

// ptr helper for s3 API calls that expect *string
func ptr(s string) *string { return &s }

// getFileStat returns fileID for a path (or nil on error).
func getFileStat(path string) *fileID {
	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}
	statT, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return &fileID{Dev: uint64(statT.Dev), Ino: uint64(statT.Ino)}
}

// getStat returns fileID for an already opened os.File.
func getStat(f *os.File) fileID {
	fi, err := f.Stat()
	if err != nil {
		return fileID{}
	}
	statT, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}
	}
	return fileID{Dev: uint64(statT.Dev), Ino: uint64(statT.Ino)}
}

// loadOffsetFromFile reads an offset file containing "<dev> <ino> <offset>".
// Returns dev, ino, offset.
func loadOffsetFromFile(offsetPath string) (dev, ino uint64, off int64, err error) {
	b, err := os.ReadFile(offsetPath)
	if err != nil {
		return 0, 0, 0, err
	}
	var dd, ii, oo uint64
	n, err := fmt.Sscanf(string(b), "%d %d %d", &dd, &ii, &oo)
	if err != nil || n != 3 {
		return 0, 0, 0, fmt.Errorf("bad offset file")
	}
	return dd, ii, int64(oo), nil
}

// saveOffsetToFile writes "<dev> <ino> <offset>" into offsetPath.
func saveOffsetToFile(offsetPath string, dev, ino uint64, off int64) error {
	if err := os.MkdirAll(filepath.Dir(offsetPath), 0o755); err != nil {
		return err
	}
	data := fmt.Sprintf("%d %d %d", dev, ino, off)
	return os.WriteFile(offsetPath, []byte(data), 0o644)
}

// normalizePodName strips common k8s suffixes (uid/replica) so folder groups by app.
// e.g. "myapp-7fd88c6-xyz" -> "myapp"
func normalizePodName(pod string) string {
	parts := strings.Split(pod, "-")
	if len(parts) < 3 {
		return pod
	}
	isHex := func(s string) bool {
		if len(s) < 6 {
			return false
		}
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}
	isAlnum := func(s string) bool {
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return false
			}
		}
		return true
	}
	last := parts[len(parts)-1]
	second := parts[len(parts)-2]
	if isHex(strings.ToLower(second)) && isAlnum(last) {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	if isAlnum(last) && len(last) <= 10 {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return pod
}
