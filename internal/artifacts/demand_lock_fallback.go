//go:build !windows && !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly && !aix

package artifacts

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Fallback for platforms without a supported advisory file lock syscall.
func withFileLock(path string, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	ownerToken := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())

	for {
		lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			if _, writeErr := lockFile.WriteString(ownerToken + "\n"); writeErr != nil {
				cleanupErr := releaseFallbackDemandLock(lockFile, path, ownerToken)
				return combineDemandLockResult(fmt.Errorf("write demand lock: %w", writeErr), cleanupErr)
			}
			if syncErr := lockFile.Sync(); syncErr != nil {
				cleanupErr := releaseFallbackDemandLock(lockFile, path, ownerToken)
				return combineDemandLockResult(fmt.Errorf("sync demand lock: %w", syncErr), cleanupErr)
			}

			fnErr := fn()
			cleanupErr := releaseFallbackDemandLock(lockFile, path, ownerToken)
			return combineDemandLockResult(fnErr, cleanupErr)
		}
		if !os.IsExist(err) {
			return fmt.Errorf("open demand lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for demand lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func releaseFallbackDemandLock(lockFile *os.File, path, ownerToken string) error {
	closeErr := lockFile.Close()

	var ownerErr error
	body, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		if strings.TrimSpace(string(body)) != ownerToken {
			ownerErr = fmt.Errorf("release demand lock: owner token mismatch")
		}
	case !os.IsNotExist(readErr):
		ownerErr = fmt.Errorf("read demand lock: %w", readErr)
	}

	var removeErr error
	if ownerErr == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			removeErr = fmt.Errorf("release demand lock: %w", err)
		}
	}

	return joinDemandLockCleanupErrors(
		wrapDemandLockCloseError(closeErr),
		ownerErr,
		removeErr,
	)
}
