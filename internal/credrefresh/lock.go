package credrefresh

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// claudeLockPath returns the exact lockfile path Claude Code locks on its
// OAuth-refresh path, so the daemon serializes against Claude's own writes.
//
// Verified in the shipped binary (cli.js): the refresh path calls
// proper-lockfile's lock() with the CONFIG_DIR (the dir holding
// .credentials.json), with no options. proper-lockfile resolves realpath() of
// its argument (realpath:true by default) and appends ".lock", so the lock dir
// is `realpath(CONFIG_DIR)+".lock"` — a SIBLING of the profile dir
// (e.g. ~/.claude.lock), NOT ~/.claude/.credentials.json.lock.
//
// credPath is the credentials FILE path; its parent dir is the CONFIG_DIR.
// EvalSymlinks resolves the dir's realpath (the dir always exists here).
func claudeLockPath(credPath string) (string, error) {
	configDir := filepath.Dir(credPath)
	resolved, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		return "", err
	}
	return resolved + ".lock", nil
}

// acquireLock takes a proper-lockfile-compatible lock so the refresh daemon
// serializes against Claude Code's own credential writes. proper-lockfile (the
// npm package Claude uses) represents a held lock as a DIRECTORY at
// `<file>.lock`: mkdir is atomic, so the first mkdir wins. A lock whose mtime
// is older than lockStaleThreshold is considered abandoned (crashed holder)
// and stolen, matching proper-lockfile's staleness recovery.
//
// Returns a release func that removes the lock dir. Blocks up to timeout
// against a fresh, actively-held lock, then returns an error rather than
// stealing it — a fresh lock means Claude is mid-refresh and we must not race.
func acquireLock(lockPath string, timeout time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			// Stamp mtime now so concurrent stealers see us as fresh.
			now := time.Now()
			_ = os.Chtimes(lockPath, now, now)
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("mkdir lock %s: %w", lockPath, err)
		}

		// Lock exists — steal it iff it is stale (abandoned by a crashed holder).
		if fi, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(fi.ModTime()) > lockStaleThreshold {
				// Best-effort steal; if another stealer beat us the next
				// mkdir simply contends again.
				_ = os.Remove(lockPath)
				continue
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock %s held and fresh; timed out after %s", lockPath, timeout)
		}
		time.Sleep(lockRetryInterval)
	}
}

// atomicWriteFile writes data to path via a temp file in the same directory
// then renames atomically. rename(2) does not follow the destination, so a
// concurrent reader never sees a torn file and a symlink at path is replaced
// rather than dereferenced. The temp file is created with perm so the secret
// is never briefly world-readable.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
