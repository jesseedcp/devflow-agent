//go:build !windows

package artifacts

func pathIsReparsePoint(path string) (bool, error) {
	return false, nil
}
