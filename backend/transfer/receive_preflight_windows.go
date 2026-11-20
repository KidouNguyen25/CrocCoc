//go:build windows

package transfer

import "os"

// PreflightReceiveDestination checks the manifest, candidate final/staging
// path lengths, local NTFS directory identity, user quota and root-name
// conflicts BEFORE showing an offer or requesting any payload. The returned
// lease must be closed by its caller. On failure all handles are released.
//
// Security boundary: name enumeration is read-only and by path; this pass
// does not prove NTFS short-name aliases, guarantee disk capacity, or guard
// against changes between checks. A handle-anchored staging/publish backend
// must revalidate everything before granting any payload-write capability.
func PreflightReceiveDestination(destination string, entries []ManifestEntry) (_ *DestinationLease, err error) {
	total, err := ValidateManifest(entries)
	if err != nil {
		return nil, err
	}
	if err := CheckReceivePathBudget(destination, entries); err != nil {
		return nil, err
	}
	lease, err := OpenReceiveDestinationLease(destination)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = lease.Close()
		}
	}()

	if err := lease.CheckCapacity(total); err != nil {
		return nil, err
	}
	listing, err := os.ReadDir(destination)
	if err != nil {
		return nil, ErrUnsupportedDestination
	}
	names := make([]string, 0, len(listing))
	for _, item := range listing {
		names = append(names, item.Name())
	}
	if err := CheckReceiveRootConflicts(entries, names); err != nil {
		return nil, err
	}
	if err := lease.VerifyCurrentPath(); err != nil {
		return nil, err
	}
	return lease, nil
}
