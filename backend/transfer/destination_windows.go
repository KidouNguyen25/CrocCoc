//go:build windows

package transfer

import (
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// CheckReceiveDestinationEligibility is the FIRST, read-only Windows gate.
// MVP allows only absolute directories on fixed local NTFS drives. It checks
// every existing component for reparse points and rejects all other paths.
//
// CRITICAL: checks by path are subject to concurrent replacement. This is NOT
// a safe write authorization or a final preflight: future native storage
// must keep directory handles, compare identities, enforce quotas, create
// staging on the same volume and publish using an atomic no-replace rename.
func CheckReceiveDestinationEligibility(directory string) error {
	if directory == "" || !filepath.IsAbs(directory) {
		return ErrUnsupportedDestination
	}
	clean := filepath.Clean(directory)
	volume := filepath.VolumeName(clean)
	if len(volume) != 2 || volume[1] != ':' ||
		!((volume[0] >= 'a' && volume[0] <= 'z') ||
			(volume[0] >= 'A' && volume[0] <= 'Z')) {
		return ErrUnsupportedDestination
	}
	units := 1
	for _, r := range clean {
		units += utf16.RuneLen(r)
		if units >= maxWindowsPathUnit {
			return ErrUnsupportedDestination
		}
	}

	driveRoot := volume + "\\"
	drivePtr, err := windows.UTF16PtrFromString(driveRoot)
	if err != nil || windows.GetDriveType(drivePtr) != windows.DRIVE_FIXED {
		return ErrUnsupportedDestination
	}
	var fsName [32]uint16
	if err := windows.GetVolumeInformation(
		drivePtr, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName)),
	); err != nil || !strings.EqualFold(windows.UTF16ToString(fsName[:]), "NTFS") {
		return ErrUnsupportedDestination
	}

	if !isPlainExistingDirectory(driveRoot) {
		return ErrUnsupportedDestination
	}
	relative := strings.TrimLeft(clean[len(volume):], "\\/")
	if relative == "" {
		return nil
	}
	current := driveRoot
	for _, part := range strings.Split(relative, "\\") {
		if part == "" || part == "." || part == ".." {
			return ErrUnsupportedDestination
		}
		current = filepath.Join(current, part)
		if !isPlainExistingDirectory(current) {
			return ErrUnsupportedDestination
		}
	}
	return nil
}

func isPlainExistingDirectory(path string) bool {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(ptr)
	return err == nil &&
		attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0 &&
		attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0
}
