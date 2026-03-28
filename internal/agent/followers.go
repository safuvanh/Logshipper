package agent

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// scanAndStartFollowers scans /var/log/containers (or configured dir) and starts followers for symlink files.
func (a *Agent) scanAndStartFollowers() error {
	// log namespaces being watched (supports single or comma-separated list)
	if a != nil && a.cfg != nil {
		names := a.cfg.TargetNSList
		if len(names) == 0 {
			names = []string{a.cfg.TargetNS}
		}
		log.Printf("watching namespaces: %v", names)
	}

	dir := a.cfg.ContainersDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		link := filepath.Join(dir, name)
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		a.startFollowerIfNeeded(name, target)
	}
	return nil
}

// startFollowerIfNeeded starts a tail follower for the given container symlink if it's a target app log.
func (a *Agent) startFollowerIfNeeded(symlinkName, targetPath string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.followers[targetPath]; ok {
		return
	}
	parts := strings.SplitN(symlinkName, "_", 3)
	if len(parts) < 3 {
		return
	}
	pod := parts[0]
	ns := parts[1]
	containerFull := parts[2]
	containerClean := containerFull
	if idx := strings.Index(containerClean, ".log"); idx >= 0 {
		containerClean = containerClean[:idx]
	}

	// Use matchesNamespace to support single or multiple namespaces (and wildcard "*")
	if !a.matchesNamespace(ns) {
		return
	}

	for _, ex := range strings.Split(a.cfg.ExcludeList, ",") {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		if strings.Contains(strings.ToLower(containerClean), strings.ToLower(ex)) {
			return
		}
	}

	app := normalizePodName(pod)

	state := &followState{cancel: make(chan struct{}), done: make(chan struct{})}
	a.followers[targetPath] = state
	go a.followFile(targetPath, pod, app, ns, state)
	log.Printf("started follower -> %s pod=%s app=%s ns=%s", targetPath, pod, app, ns)
}

// followFile tails a single container file, resumes from offset when possible, and writes application lines.
func (a *Agent) followFile(targetPath, pod, appName, ns string, state *followState) {
	defer func() {
		// ensure follower map cleanup happens if followFile ends because file removed/rotated
		a.mu.Lock()
		delete(a.followers, targetPath)
		a.mu.Unlock()
		close(state.done)
		log.Printf("stopped follower -> %s pod=%s app=%s ns=%s", targetPath, pod, appName, ns)
	}()

	var f *os.File
	var err error

	safeName := strings.ReplaceAll(filepath.Base(targetPath), string(filepath.Separator), "_")
	offsetPath := filepath.Join(a.cfg.TmpDir, "offsets", ns, pod+"__"+safeName+".offset")
	// key used for watchdog maps
	key := ns + "/" + appName + "/" + pod

	for {
		select {
		case <-state.cancel:
			if f != nil {
				_ = f.Close()
			}
			return
		default:
		}

		f, err = os.Open(targetPath)
		if err != nil {
			log.Printf("ERROR: cannot open %s: %v", targetPath, err)
			time.Sleep(400 * time.Millisecond)
			continue
		}

		// mark initial activity when follower starts (prevents immediate watchdog warnings)
		a.mu.Lock()
		a.lastActivity[key] = time.Now()
		if a.warned[key] {
			a.warned[key] = false
		}
		a.mu.Unlock()

		stat := getStat(f)

		// resume using offsets or default tail-only
		if dev, ino, off, err := loadOffsetFromFile(offsetPath); err == nil {
			if dev == stat.Dev && ino == stat.Ino {
				if off >= 0 {
					_, _ = f.Seek(off, io.SeekStart)
				} else {
					_, _ = f.Seek(0, io.SeekEnd)
				}
			} else {
				// different inode -> start at beginning (policy)
				_, _ = f.Seek(0, io.SeekStart)
			}
		} else {
			// no offset -> default tail (skip existing)
			_, _ = f.Seek(0, io.SeekEnd)
		}

		reader := bufio.NewReader(f)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					curStat := getStat(f)
					pos, _ := f.Seek(0, io.SeekCurrent)
					_ = saveOffsetToFile(offsetPath, curStat.Dev, curStat.Ino, pos)

					newStat := getFileStat(targetPath)
					if newStat != nil {
						if newStat.Ino != stat.Ino || newStat.Dev != stat.Dev {
							_ = f.Close()
							break
						}
						time.Sleep(150 * time.Millisecond)
						continue
					} else {
						_ = f.Close()
						break
					}
				} else {
					fmt.Fprintf(os.Stderr, "read err %s: %v\n", targetPath, err)
					_ = f.Close()
					break
				}
			}

			// process the log line (writes to per-pod tmp file)
			// Pass state to track last timestamp for multi-line logs
			a.processLine(appName, pod, ns, line, state)

			// update watchdog last-activity timestamp and clear any previous warning
			a.mu.Lock()
			a.lastActivity[key] = time.Now()
			if a.warned[key] {
				a.warned[key] = false
			}
			a.mu.Unlock()
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// watchContainersDir watches the containers directory for new/removed symlinks and starts/stops followers.
func (a *Agent) watchContainersDir() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fsnotify init err: %v\n", err)
		return
	}
	defer w.Close()
	if err := w.Add(a.cfg.ContainersDir); err != nil {
		fmt.Fprintf(os.Stderr, "watch add containers err: %v\n", err)
		return
	}
	for {
		select {
		case ev := <-w.Events:
			name := filepath.Base(ev.Name)
			if ev.Op&fsnotify.Create == fsnotify.Create || ev.Op&fsnotify.Write == fsnotify.Write {
				link := filepath.Join(a.cfg.ContainersDir, name)
				target, err := os.Readlink(link)
				if err != nil {
					continue
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(a.cfg.ContainersDir, target)
				}
				a.startFollowerIfNeeded(name, target)
			}
			if ev.Op&fsnotify.Remove == fsnotify.Remove || ev.Op&fsnotify.Rename == fsnotify.Rename {
				a.mu.Lock()
				for targetPath, st := range a.followers {
					parts := strings.SplitN(name, "_", 3)
					if len(parts) < 3 {
						continue
					}
					pod := parts[0]
					ns := parts[1]
					if strings.Contains(targetPath, pod) && strings.Contains(targetPath, ns) {
						// signal follower to stop and remove from map
						close(st.cancel)
						delete(a.followers, targetPath)
						log.Printf("stopped follower (symlink removed) -> %s", targetPath)
					}
				}
				a.mu.Unlock()
			}
		case err := <-w.Errors:
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		}
	}
}

// watchPodsDir watches the pods directory tree (adds watchers for new pod dirs).
func (a *Agent) watchPodsDir() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fsnotify pods err: %v\n", err)
		return
	}
	defer w.Close()
	_ = w.Add(a.cfg.PodsDir)
	for {
		select {
		case ev := <-w.Events:
			if ev.Op&fsnotify.Create == fsnotify.Create {
				fi, err := os.Stat(ev.Name)
				if err == nil && fi.IsDir() {
					_ = w.Add(ev.Name)
				}
			}
		case err := <-w.Errors:
			fmt.Fprintf(os.Stderr, "pods watch err: %v\n", err)
		}
	}
}
