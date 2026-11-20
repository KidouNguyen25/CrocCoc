//go:build windows

package transfer

// CheckReceiveDestinationEligibility performs the read-only destination
// check and promptly releases its handles. A receiver MUST acquire a new
// DestinationLease and keep it through further native preflight/operations;
// this convenience check alone never authorizes a transfer.
func CheckReceiveDestinationEligibility(directory string) error {
	lease, err := OpenReceiveDestinationLease(directory)
	if err != nil {
		return err
	}
	return lease.Close()
}
