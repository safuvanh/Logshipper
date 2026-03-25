package agent

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)


func (a *Agent) uploadLoop() {
	for {
		a.uploadOnce()
		time.Sleep(time.Duration(a.cfg.IntervalSec) * time.Second)
	}
}

func (a *Agent) uploadOnce() {
	cfg := a.cfg
	_ = filepath.Walk(cfg.TmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if info.IsDir() { return nil }
		if !strings.HasSuffix(info.Name(), ".log") { return nil }

		rel, _ := filepath.Rel(cfg.TmpDir, path)
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 3 { return nil }
		ns := parts[0]
		appName := parts[1]
		filename := parts[2]
		if len(filename) < 15 { return nil }
		datePart := filename[len(filename)-14:len(filename)-4]
		podName := filename[:len(filename)-len("-"+datePart+".log")]


		dateParts := strings.SplitN(datePart, "-", 3)
		if len(dateParts) != 3 { return nil }
		year, monthNum, day := dateParts[0], dateParts[1], dateParts[2]
		mi, err := strconv.Atoi(monthNum)
		if err != nil || mi < 1 || mi > 12 { return nil }
		monthName := time.Month(mi).String() // "January", "February", etc.
		key := fmt.Sprintf("%s/%s/%s/%s/%s/%s-%s.log.gz", ns, appName, year, monthName, day, podName, datePart)

		gzPath, md5hex, err := compressAndMD5(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "compress err: %v\n", err)
			return nil
		}
		defer func() { _ = os.Remove(gzPath) }()

		a.mu.Lock()
		prev, saw := a.checksums[key]
		a.mu.Unlock()
		if saw && prev == md5hex {
			log.Printf("skip upload no-change %s\n", key)
			return nil
		}

		if err := a.uploadGzipPath(gzPath, key, md5hex); err != nil {
			fmt.Fprintf(os.Stderr, "upload err %s: %v\n", key, err)
			return nil
		}

		// CRITICAL: Use UTC to match log timestamps (logs are always in UTC)
		today := time.Now().UTC().Format("2006-01-02")
		if datePart != today {
			_ = os.Remove(path)
		}
		log.Printf("uploaded %s\n", key)
		return nil
	})
}

func compressAndMD5(srcPath string) (string, string, error) {
	dir := filepath.Dir(srcPath)
	base := filepath.Base(srcPath)
	tmpGz := filepath.Join(dir, base+".tmp.gz")
	finalGz := filepath.Join(dir, base+".gz")

	sf, err := os.Open(srcPath)
	if err != nil { return "", "", err }
	defer sf.Close()

	gf, err := os.Create(tmpGz)
	if err != nil { return "", "", err }
	gw := gzip.NewWriter(gf)
	if _, err := io.Copy(gw, sf); err != nil {
		_ = gw.Close(); _ = gf.Close(); _ = os.Remove(tmpGz)
		return "", "", err
	}
	_ = gw.Close(); _ = gf.Close()

	_ = os.Remove(finalGz)
	if err := os.Rename(tmpGz, finalGz); err == nil {
		// ok
	} else {
		finalGz = tmpGz
	}

	gf2, err := os.Open(finalGz)
	if err != nil { return "", "", err }
	defer gf2.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, gf2); err != nil { return "", "", err }
	sum := hash.Sum(nil)
	return finalGz, hex.EncodeToString(sum), nil
}

func (a *Agent) uploadGzipPath(gzPath, key, md5hex string) error {
	f, err := os.Open(gzPath)
	if err != nil { return fmt.Errorf("open gz %s: %w", gzPath, err) }
	defer f.Close()
	fi, err := f.Stat()
	if err != nil { return fmt.Errorf("stat gz %s: %w", gzPath, err) }
	size := fi.Size()

	cfg := a.cfg
	_, err = a.s3client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        &cfg.S3Bucket,
		Key:           &key,
		Body:          f,
		ContentType:   ptr("application/gzip"),
		ContentLength: size,
	})
	if err != nil { return fmt.Errorf("s3 putobject: %w", err) }

	a.mu.Lock()
	a.checksums[key] = md5hex
	a.mu.Unlock()
	return nil
}
