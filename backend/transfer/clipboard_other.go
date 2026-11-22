//go:build !windows

package transfer

// CopySecretToProtectedClipboard fails closed on non-Windows hosts where
// Win32 privacy clipboard markers are unavailable.
func CopySecretToProtectedClipboard(string) error {
	return ErrCopyUnsafe
}
