package transfer

import (
	"errors"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	MaxManifestEntries = 10_000
	MaxManifestBytes   = int64(20) << 30
	maxWindowsPathUnit = 260 // Conservative MVP limit, including the terminating NUL.
)

var (
	ErrUnsafeOffer   = errors.New("UNSAFE_OFFER: danh sách tệp không an toàn")
	ErrResourceLimit = errors.New("RESOURCE_LIMIT: vượt giới hạn nhận tệp")
)

// ManifestEntry is untrusted peer metadata. A directory declares no payload.
type ManifestEntry struct {
	Path      string
	Directory bool
	SizeBytes int64
}

// ValidateManifest checks the lexical subset of receive preflight only.
// The caller MUST additionally verify NTFS aliases, destination identity,
// reparse points, capacity and each actual write using OS handles before
// requesting any payload.
func ValidateManifest(entries []ManifestEntry) (int64, error) {
	if len(entries) == 0 {
		return 0, ErrUnsafeOffer
	}
	if len(entries) > MaxManifestEntries {
		return 0, ErrResourceLimit
	}
	seen := make(map[string]bool, len(entries))
	var total int64
	for _, entry := range entries {
		parts, ok := safeRelativePath(entry.Path)
		if !ok || entry.SizeBytes < 0 || (entry.Directory && entry.SizeBytes != 0) {
			return 0, ErrUnsafeOffer
		}
		key := strings.ToUpper(strings.Join(parts, "/"))
		if _, duplicate := seen[key]; duplicate {
			return 0, ErrUnsafeOffer
		}
		seen[key] = entry.Directory
		if entry.SizeBytes > MaxManifestBytes-total {
			return 0, ErrResourceLimit
		}
		total += entry.SizeBytes
	}
	for key := range seen {
		for pos := strings.IndexByte(key, '/'); pos != -1; {
			if dir, exists := seen[key[:pos]]; exists && !dir {
				return 0, ErrUnsafeOffer
			}
			next := strings.IndexByte(key[pos+1:], '/')
			if next < 0 {
				break
			}
			pos += next + 1
		}
	}
	return total, nil
}

func safeRelativePath(path string) ([]string, bool) {
	if path == "" || !utf8.ValidString(path) {
		return nil, false
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return nil, false
	}
	// Win32 MAX_PATH is counted in UTF-16 units, not UTF-8 bytes.
	units := 1
	for _, r := range normalized {
		units += utf16.RuneLen(r)
		if units >= maxWindowsPathUnit {
			return nil, false
		}
	}
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if !safeWindowsComponent(part) {
			return nil, false
		}
	}
	return parts, true
}

func safeWindowsComponent(part string) bool {
	if part == "" || part == "." || part == ".." ||
		strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
		return false
	}
	for _, r := range part {
		if r < 0x20 || r == 0x7f || strings.ContainsRune("<>:\"|?*", r) ||
			r == 0x061c || r == 0x200e || r == 0x200f ||
			(r >= 0x202a && r <= 0x202e) ||
			(r >= 0x2066 && r <= 0x2069) {
			return false
		}
	}
	base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	runes := []rune(base)
	if len(runes) == 4 {
		prefix, digit := string(runes[:3]), runes[3]
		if (prefix == "COM" || prefix == "LPT") &&
			((digit >= '1' && digit <= '9') || digit == '¹' || digit == '²' || digit == '³') {
			return false
		}
	}
	return true
}
