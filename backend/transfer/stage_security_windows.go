//go:build windows

package transfer

import "golang.org/x/sys/windows"

// privateStageSecurity is supplied at native creation, so the anonymous
// staging name never exists with permissions inherited from a shared parent.
// Only the host process user and LocalSystem are granted ordinary access.
// This is not isolation from another process running as that same user.
func privateStageSecurity() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil || !user.User.Sid.IsValid() {
		return nil, ErrUnsafeStorage
	}
	sid := user.User.Sid.String()
	sddl := "D:P(A;OICI;FA;;;SY)"
	if sid != "S-1-5-18" {
		sddl += "(A;OICI;FA;;;" + sid + ")"
	}
	security, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil || security == nil || !security.IsValid() {
		return nil, ErrUnsafeStorage
	}
	return security, nil
}
