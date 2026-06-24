//go:build windows

package artifacts

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	lockFileExclusiveLock                      = 0x00000002
	lockFileFailImmediately                    = 0x00000001
	windowsErrorLockViolation    syscall.Errno = 33
	windowsErrorSharingViolation syscall.Errno = 32
	windowsErrorAccessDenied     syscall.Errno = 5
	windowsErrorIOPending        syscall.Errno = 997
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

func withFileLock(path string, timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)

	for {
		lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			if isWindowsDemandLockOpenContention(path, err) {
				if time.Now().After(deadline) {
					return fmt.Errorf("timed out waiting for demand lock")
				}
				time.Sleep(25 * time.Millisecond)
				continue
			}
			return fmt.Errorf("open demand lock: %w", err)
		}

		overlapped := &syscall.Overlapped{}
		err = lockWindowsDemandFile(lockFile, overlapped)
		if err == nil {
			fnErr := fn()
			cleanupErr := unlockAndCloseWindowsDemandLock(lockFile, overlapped)
			return combineDemandLockResult(fnErr, cleanupErr)
		}

		closeErr := lockFile.Close()
		if isWindowsDemandLockContention(err) {
			if closeErr != nil {
				return fmt.Errorf("close demand lock: %w", closeErr)
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for demand lock")
			}
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return combineDemandLockResult(fmt.Errorf("lock demand lock: %w", err), wrapDemandLockCloseError(closeErr))
	}
}

func lockWindowsDemandFile(lockFile *os.File, overlapped *syscall.Overlapped) error {
	r1, _, callErr := procLockFileEx.Call(
		lockFile.Fd(),
		uintptr(lockFileExclusiveLock|lockFileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 != 0 {
		return nil
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return syscall.EINVAL
}

func unlockAndCloseWindowsDemandLock(lockFile *os.File, overlapped *syscall.Overlapped) error {
	unlockErr := unlockWindowsDemandFile(lockFile, overlapped)
	closeErr := lockFile.Close()
	return joinDemandLockCleanupErrors(
		wrapDemandLockUnlockError(unlockErr),
		wrapDemandLockCloseError(closeErr),
	)
}

func unlockWindowsDemandFile(lockFile *os.File, overlapped *syscall.Overlapped) error {
	r1, _, callErr := procUnlockFileEx.Call(
		lockFile.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 != 0 {
		return nil
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return syscall.EINVAL
}

func isWindowsDemandLockOpenContention(path string, err error) bool {
	if !errors.Is(err, windowsErrorAccessDenied) && !errors.Is(err, windowsErrorSharingViolation) {
		return false
	}
	_, statErr := os.Stat(path)
	return statErr == nil
}

func isWindowsDemandLockContention(err error) bool {
	return errors.Is(err, windowsErrorLockViolation) || errors.Is(err, windowsErrorIOPending)
}
