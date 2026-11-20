package transfer

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidInput      = errors.New("INVALID_INPUT: cần chọn nguồn hợp lệ")
	ErrSourceUnavailable = errors.New("SOURCE_UNAVAILABLE: không thể đọc nguồn an toàn")
)

// InspectSendSources constructs a bounded, provisional manifest from local
// files. It rejects symlinks, special files, links in the source's ancestors,
// duplicate names and unsupported Windows relative names.
//
// This scan is NOT a stable read lease: file and ancestor identities MUST be
// held/rechecked through native handles before and during engine reads.
// Never authorize real transmission based on this result alone.
func InspectSendSources(paths []string) ([]ManifestEntry, error) {
	if len(paths) == 0 {
		return nil, ErrInvalidInput
	}
	entries := make([]ManifestEntry, 0)
	var total int64
	for _, selected := range paths {
		if selected == "" || strings.TrimSpace(selected) == "" {
			return nil, ErrInvalidInput
		}
		root, err := filepath.Abs(selected)
		if err != nil || !plainSourceAncestors(root) {
			return nil, ErrSourceUnavailable
		}
		name := filepath.Base(root)
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return ErrSourceUnavailable
			}
			info, err := os.Lstat(path)
			if err != nil || info.Mode()&os.ModeSymlink != 0 ||
				(!info.IsDir() && !info.Mode().IsRegular()) {
				return ErrSourceUnavailable
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return ErrSourceUnavailable
			}
			wirePath := name
			if rel != "." {
				wirePath += "/" + filepath.ToSlash(rel)
			}
			size := int64(0)
			if !info.IsDir() {
				size = info.Size()
				if size < 0 || size > MaxManifestBytes-total {
					return ErrResourceLimit
				}
			}
			if len(entries) >= MaxManifestEntries {
				return ErrResourceLimit
			}
			entries = append(entries, ManifestEntry{
				Path: wirePath, Directory: info.IsDir(), SizeBytes: size,
			})
			total += size
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if _, err := ValidateManifest(entries); err != nil {
		if errors.Is(err, ErrResourceLimit) {
			return nil, err
		}
		return nil, ErrInvalidInput
	}
	return entries, nil
}

func plainSourceAncestors(path string) bool {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
		current = parent
	}
}
