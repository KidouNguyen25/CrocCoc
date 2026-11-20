package transfer

import (
	"io/fs"
	"sort"
	"strings"
)

// CrocReceiveMetadata is the narrow metadata boundary for the pinned Croc
// adapter. It contains no peer paths other than intended relative output
// names, and no connection code, local paths or protocol credentials.
type CrocReceiveMetadata struct {
	Files            []CrocReceiveFile
	EmptyDirectories []string
	SendingText      bool
}

type CrocReceiveFile struct {
	Folder    string
	Name      string
	SizeBytes int64
	Mode      fs.FileMode
	Symlink   bool
	Archive   bool
}

// ProjectCrocReceiveManifest captures the exact directory tree represented by
// Croc's regular file list and empty-folder list. The result is metadata only
// and never permits starting a payload transfer.
func ProjectCrocReceiveManifest(metadata CrocReceiveMetadata) ([]ManifestEntry, error) {
	if metadata.SendingText {
		return nil, ErrUnsafeOffer
	}
	if len(metadata.Files) > MaxManifestEntries ||
		len(metadata.EmptyDirectories) > MaxManifestEntries-len(metadata.Files) {
		return nil, ErrResourceLimit
	}
	type entryState struct {
		entry ManifestEntry
		empty bool
	}
	all := make(map[string]entryState, len(metadata.Files)+len(metadata.EmptyDirectories))

	add := func(path string, directory, empty bool, size int64) error {
		key := strings.ToUpper(path)
		if current, exists := all[key]; exists {
			if directory && !empty && current.entry.Directory && !current.empty && current.entry.Path == path {
				return nil // Another file implies the same parent directory.
			}
			return ErrUnsafeOffer
		}
		if len(all) == MaxManifestEntries {
			return ErrResourceLimit
		}
		all[key] = entryState{
			entry: ManifestEntry{Path: path, Directory: directory, SizeBytes: size},
			empty: empty,
		}
		return nil
	}
	parents := func(parts []string) error {
		var path string
		for _, component := range parts {
			if path != "" {
				path += "/"
			}
			path += component
			if err := add(path, true, false, 0); err != nil {
				return err
			}
		}
		return nil
	}
	var total int64
	for _, file := range metadata.Files {
		if file.Symlink || file.Archive || !file.Mode.IsRegular() || !safeWindowsComponent(file.Name) ||
			file.SizeBytes < 0 {
			return nil, ErrUnsafeOffer
		}
		if file.SizeBytes > MaxManifestBytes-total {
			return nil, ErrResourceLimit
		}
		total += file.SizeBytes
		folder := strings.ReplaceAll(file.Folder, "\\", "/")
		var path string
		switch folder {
		case "", ".", "./":
			path = file.Name
		default:
			path = strings.TrimSuffix(folder, "/") + "/" + file.Name
		}
		parts, ok := safeRelativePath(path)
		if !ok {
			return nil, ErrUnsafeOffer
		}
		if err := parents(parts[:len(parts)-1]); err != nil {
			return nil, err
		}
		if err := add(path, false, false, file.SizeBytes); err != nil {
			return nil, err
		}
	}
	for _, folder := range metadata.EmptyDirectories {
		path := strings.TrimSuffix(strings.ReplaceAll(folder, "\\", "/"), "/")
		parts, ok := safeRelativePath(path)
		if !ok {
			return nil, ErrUnsafeOffer
		}
		if err := parents(parts[:len(parts)-1]); err != nil {
			return nil, err
		}
		if err := add(path, true, true, 0); err != nil {
			return nil, err
		}
	}
	result := make([]ManifestEntry, 0, len(all))
	for _, item := range all {
		result = append(result, item.entry)
	}
	if _, err := ValidateManifest(result); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}
