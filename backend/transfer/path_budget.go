package transfer

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Conservative template for the fixed-length UUID staging directory.
// Actual native storage must validate its generated path again at creation.
const stagingDirectoryTemplate = ".croccoc-stage-00000000-0000-0000-0000-000000000000"

// CheckReceivePathBudget checks BOTH the final and proposed staging locations
// against the MVP's 260 UTF-16-unit limit, including NUL. The destination is
// an absolute Windows drive path and entries contain only relative names.
//
// This lexical helper cannot authorize any write: the destination must still
// be opened by native handles, checked for NTFS aliases and re-verified at
// staging creation and the no-replace publish operation.
func CheckReceivePathBudget(destination string, entries []ManifestEntry) error {
	if _, err := ValidateManifest(entries); err != nil {
		return err
	}
	normalized, ok := normalizeWindowsDestination(destination)
	if !ok {
		return ErrUnsupportedDestination
	}
	for _, entry := range entries {
		relative := strings.ReplaceAll(entry.Path, "\\", "/")
		final := normalized + "/" + relative
		staged := normalized + "/" + stagingDirectoryTemplate + "/payload/" + relative
		if windowsPathUnits(final) >= maxWindowsPathUnit ||
			windowsPathUnits(staged) >= maxWindowsPathUnit {
			return ErrUnsafeOffer
		}
	}
	return nil
}

func normalizeWindowsDestination(directory string) (string, bool) {
	if !utf8.ValidString(directory) || len(directory) < 3 {
		return "", false
	}
	path := strings.ReplaceAll(directory, "\\", "/")
	drive := path[0]
	if !((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) ||
		path[1] != ':' || path[2] != '/' {
		return "", false
	}
	// Do not normalize traversal or duplicate separators here: a check
	// operating on a different path string than native code must fail closed.
	path = strings.TrimSuffix(path, "/")
	if len(path) == 2 { // C:/ root
		return path, true
	}
	for _, part := range strings.Split(path[3:], "/") {
		if !safeWindowsComponent(part) {
			return "", false
		}
	}
	return path, true
}

func windowsPathUnits(path string) int {
	units := 1 // NUL terminator
	for _, r := range path {
		units += utf16.RuneLen(r)
	}
	return units
}
