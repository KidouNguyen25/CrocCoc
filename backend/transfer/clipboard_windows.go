//go:build windows

package transfer

import (
	"encoding/binary"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText            = 13
	gmemMoveable             = 0x0002
	hwndMessage              = ^uintptr(2) // HWND_MESSAGE = (HWND)-3
	clipboardOpenRetries     = 5
	clipboardOpenRetryDelay  = 10 * time.Millisecond
	clipboardExcludeFormat   = "ExcludeClipboardContentFromMonitorProcessing"
	clipboardNoHistoryFormat = "CanIncludeInClipboardHistory"
	clipboardNoCloudFormat   = "CanUploadToCloudClipboard"
)

var (
	user32DLL                 = windows.NewLazySystemDLL("user32.dll")
	kernel32DLL               = windows.NewLazySystemDLL("kernel32.dll")
	procCreateWindowExW       = user32DLL.NewProc("CreateWindowExW")
	procDestroyWindow         = user32DLL.NewProc("DestroyWindow")
	procRegisterClipboardFmtW = user32DLL.NewProc("RegisterClipboardFormatW")
	procOpenClipboard         = user32DLL.NewProc("OpenClipboard")
	procCloseClipboard        = user32DLL.NewProc("CloseClipboard")
	procEmptyClipboard        = user32DLL.NewProc("EmptyClipboard")
	procSetClipboardData      = user32DLL.NewProc("SetClipboardData")
	procGlobalAlloc           = kernel32DLL.NewProc("GlobalAlloc")
	procGlobalLock            = kernel32DLL.NewProc("GlobalLock")
	procGlobalUnlock          = kernel32DLL.NewProc("GlobalUnlock")
	procGlobalFree            = kernel32DLL.NewProc("GlobalFree")
	procRtlMoveMemory         = kernel32DLL.NewProc("RtlMoveMemory")
)

func createClipboardOwnerWindow() (uintptr, error) {
	className, err := windows.UTF16PtrFromString("STATIC")
	if err != nil {
		return 0, ErrCopyUnsafe
	}
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		hwndMessage,
		0, 0, 0,
	)
	runtime.KeepAlive(className)
	if hwnd == 0 {
		return 0, ErrCopyUnsafe
	}
	return hwnd, nil
}

func registerClipboardFormat(name string) (uintptr, error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, ErrCopyUnsafe
	}
	id, _, _ := procRegisterClipboardFmtW.Call(uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
	if id == 0 {
		return 0, ErrCopyUnsafe
	}
	return id, nil
}

func allocGlobalBytes(data []byte) (uintptr, error) {
	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if hMem == 0 {
		return 0, ErrCopyUnsafe
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return 0, ErrCopyUnsafe
	}
	if len(data) > 0 {
		procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
		runtime.KeepAlive(data)
	}
	procGlobalUnlock.Call(hMem)
	return hMem, nil
}

func openClipboardWithRetry(hwnd uintptr) bool {
	for i := 0; i < clipboardOpenRetries; i++ {
		if opened, _, _ := procOpenClipboard.Call(hwnd); opened != 0 {
			return true
		}
		time.Sleep(clipboardOpenRetryDelay)
	}
	return false
}

// CopySecretToProtectedClipboard writes secret to the Windows clipboard using
// an owned message-only HWND, registering and setting Windows privacy markers
// before CF_UNICODETEXT in the same open session. If any step fails before
// setting the secret, it fails closed with ErrCopyUnsafe.
func CopySecretToProtectedClipboard(secret string) error {
	if secret == "" || strings.ContainsAny(secret, "\r\n\x00") {
		return ErrCopyUnsafe
	}
	excludeFmt, err := registerClipboardFormat(clipboardExcludeFormat)
	if err != nil {
		return ErrCopyUnsafe
	}
	historyFmt, err := registerClipboardFormat(clipboardNoHistoryFormat)
	if err != nil {
		return ErrCopyUnsafe
	}
	cloudFmt, err := registerClipboardFormat(clipboardNoCloudFormat)
	if err != nil {
		return ErrCopyUnsafe
	}

	var zeroDword [4]byte
	excludeMem, err := allocGlobalBytes(zeroDword[:])
	if err != nil {
		return ErrCopyUnsafe
	}
	historyMem, err := allocGlobalBytes(zeroDword[:])
	if err != nil {
		procGlobalFree.Call(excludeMem)
		return ErrCopyUnsafe
	}
	cloudMem, err := allocGlobalBytes(zeroDword[:])
	if err != nil {
		procGlobalFree.Call(excludeMem)
		procGlobalFree.Call(historyMem)
		return ErrCopyUnsafe
	}

	units := utf16.Encode([]rune(secret))
	units = append(units, 0)
	textBytes := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(textBytes[i*2:], u)
	}
	textMem, err := allocGlobalBytes(textBytes)
	if err != nil {
		procGlobalFree.Call(excludeMem)
		procGlobalFree.Call(historyMem)
		procGlobalFree.Call(cloudMem)
		return ErrCopyUnsafe
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hwnd, err := createClipboardOwnerWindow()
	if err != nil {
		procGlobalFree.Call(excludeMem)
		procGlobalFree.Call(historyMem)
		procGlobalFree.Call(cloudMem)
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}
	defer procDestroyWindow.Call(hwnd)

	if !openClipboardWithRetry(hwnd) {
		procGlobalFree.Call(excludeMem)
		procGlobalFree.Call(historyMem)
		procGlobalFree.Call(cloudMem)
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}
	defer procCloseClipboard.Call()

	if emptied, _, _ := procEmptyClipboard.Call(); emptied == 0 {
		procGlobalFree.Call(excludeMem)
		procGlobalFree.Call(historyMem)
		procGlobalFree.Call(cloudMem)
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}

	// Markers MUST succeed before CF_UNICODETEXT is placed on the clipboard.
	if r, _, _ := procSetClipboardData.Call(excludeFmt, excludeMem); r == 0 {
		procGlobalFree.Call(excludeMem)
		procGlobalFree.Call(historyMem)
		procGlobalFree.Call(cloudMem)
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}
	if r, _, _ := procSetClipboardData.Call(historyFmt, historyMem); r == 0 {
		procEmptyClipboard.Call()
		procGlobalFree.Call(historyMem)
		procGlobalFree.Call(cloudMem)
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}
	if r, _, _ := procSetClipboardData.Call(cloudFmt, cloudMem); r == 0 {
		procEmptyClipboard.Call()
		procGlobalFree.Call(cloudMem)
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}
	if r, _, _ := procSetClipboardData.Call(cfUnicodeText, textMem); r == 0 {
		procEmptyClipboard.Call()
		procGlobalFree.Call(textMem)
		return ErrCopyUnsafe
	}
	return nil
}
