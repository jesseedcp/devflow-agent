//go:build windows

package artifacts

import "syscall"

func pathIsReparsePoint(path string) (bool, error) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}

	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return false, err
	}

	return attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
