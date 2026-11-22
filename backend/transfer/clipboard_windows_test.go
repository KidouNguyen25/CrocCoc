//go:build windows

package transfer

import (
	"errors"
	"testing"
	"unicode/utf16"
	"unsafe"
)

func TestCopySecretToProtectedClipboardWindows(t *testing.T) {
	const secret = "croccoc-win32-clipboard-check-01"
	if err := CopySecretToProtectedClipboard(""); !errors.Is(err, ErrCopyUnsafe) {
		t.Fatalf("empty secret = %v, want ErrCopyUnsafe", err)
	}
	if err := CopySecretToProtectedClipboard("bad\nsecret"); !errors.Is(err, ErrCopyUnsafe) {
		t.Fatalf("newline secret = %v, want ErrCopyUnsafe", err)
	}
	if err := CopySecretToProtectedClipboard(secret); err != nil {
		t.Fatalf("CopySecretToProtectedClipboard failed on Windows: %v", err)
	}

	excludeFmt, err := registerClipboardFormat(clipboardExcludeFormat)
	if err != nil {
		t.Fatal(err)
	}
	historyFmt, err := registerClipboardFormat(clipboardNoHistoryFormat)
	if err != nil {
		t.Fatal(err)
	}
	cloudFmt, err := registerClipboardFormat(clipboardNoCloudFormat)
	if err != nil {
		t.Fatal(err)
	}

	procIsFormatAvail := user32DLL.NewProc("IsClipboardFormatAvailable")
	procGetClipboardData := user32DLL.NewProc("GetClipboardData")
	procGlobalSize := kernel32DLL.NewProc("GlobalSize")

	for _, check := range []struct {
		name string
		fmt  uintptr
	}{
		{clipboardExcludeFormat, excludeFmt},
		{clipboardNoHistoryFormat, historyFmt},
		{clipboardNoCloudFormat, cloudFmt},
		{"CF_UNICODETEXT", cfUnicodeText},
	} {
		avail, _, _ := procIsFormatAvail.Call(check.fmt)
		if avail == 0 {
			t.Fatalf("clipboard format %s is missing", check.name)
		}
	}

	opened, _, _ := procOpenClipboard.Call(0)
	if opened == 0 {
		t.Fatal("failed to open clipboard for verification")
	}
	defer func() {
		procEmptyClipboard.Call()
		procCloseClipboard.Call()
	}()

	handle, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		t.Fatal("GetClipboardData(CF_UNICODETEXT) returned 0")
	}
	size, _, _ := procGlobalSize.Call(handle)
	ptr, _, _ := procGlobalLock.Call(handle)
	if ptr == 0 || size < 2 {
		t.Fatal("failed to lock clipboard text")
	}
	raw := make([]byte, size)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&raw[0])), ptr, size)
	procGlobalUnlock.Call(handle)

	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		u := uint16(raw[i]) | uint16(raw[i+1])<<8
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	got := string(utf16.Decode(units))
	if got != secret {
		t.Fatalf("clipboard text = %q, want %q", got, secret)
	}
}
