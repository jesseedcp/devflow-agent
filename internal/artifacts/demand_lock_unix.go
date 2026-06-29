//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly || aix

package artifacts

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func withFileLock(path string, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)

	for {
		lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return fmt.Errorf("open demand lock: %w", err)
		}

		err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			fnErr := fn()
			cleanupErr := unlockAndCloseUnixDemandLock(lockFile)
			return combineDemandLockResult(fnErr, cleanupErr)
		}

		closeErr := lockFile.Close()
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return combineDemandLockResult(fmt.Errorf("lock demand lock: %w", err), wrapDemandLockCloseError(closeErr))
		}
		if closeErr != nil {
			return fmt.Errorf("close demand lock: %w", closeErr)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for demand lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func unlockAndCloseUnixDemandLock(lockFile *os.File) error {
	unlockErr := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	closeErr := lockFile.Close()
	return joinDemandLockCleanupErrors(
		wrapDemandLockUnlockError(unlockErr),
		wrapDemandLockCloseError(closeErr),
	)
}
