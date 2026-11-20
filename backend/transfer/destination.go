package transfer

import "errors"

// ErrUnsupportedDestination is deliberately path-free to prevent disclosure
// of the receiver's local filesystem structure to UI logs or peer diagnostics.
var ErrUnsupportedDestination = errors.New("UNSUPPORTED_DESTINATION: đích nhận không được hỗ trợ")
