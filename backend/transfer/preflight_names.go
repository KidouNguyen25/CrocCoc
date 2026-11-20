package transfer

import (
	"errors"
	"strings"
)

var ErrDestinationConflict = errors.New("DESTINATION_CONFLICT: mục nhận đã có trong thư mục đích")

// CheckReceiveRootConflicts provides an EARLY lexical check for conflicts at
// top level. A directory is one root item even when it contains many entries.
// existingNames must come from the destination directory, never from the peer.
//
// IMPORTANT: this is not a no-replace or NTFS identity guarantee. Windows
// short-name aliases, reparse points, concurrent changes, and path identity
// must be checked using native handles, again at publish time.
func CheckReceiveRootConflicts(entries []ManifestEntry, existingNames []string) error {
	if _, err := ValidateManifest(entries); err != nil {
		return err
	}
	incomingRoots := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		normalized := strings.ReplaceAll(entry.Path, "\\", "/")
		root, _, _ := strings.Cut(normalized, "/")
		incomingRoots[strings.ToUpper(root)] = struct{}{}
	}
	for _, name := range existingNames {
		if _, found := incomingRoots[strings.ToUpper(name)]; found {
			return ErrDestinationConflict
		}
	}
	return nil
}
