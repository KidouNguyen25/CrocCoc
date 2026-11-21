//go:build !windows

package transfer

// NewEngineService fails closed outside supported Windows NTFS targets.
func NewEngineService(EngineServiceConfig) Service {
	return unavailableService{}
}
