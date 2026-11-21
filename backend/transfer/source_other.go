//go:build !windows

package transfer

func sourcePathIsReparsePoint(string) (bool, error) {
	return false, nil
}
