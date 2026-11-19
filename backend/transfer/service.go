package transfer

// Service is the application boundary used by the desktop bridge.
// The implementation does not depend on Wails or any user interface.
type Service interface {
	GetCrocInfo() CrocInfo
	StartSend(paths []string) (string, error)
	StartReceive(code, outputDir string) (string, error)
	CancelTransfer(transferID string) (bool, error)
}

// NewUnavailableService exposes the bootstrap behavior before Croc integration.
func NewUnavailableService() Service {
	return unavailableService{}
}

type unavailableService struct{}

func (unavailableService) GetCrocInfo() CrocInfo {
	return CrocInfo{Available: false, Message: ErrNotIntegrated.Error()}
}

func (unavailableService) StartSend([]string) (string, error) {
	return "", ErrNotIntegrated
}

func (unavailableService) StartReceive(string, string) (string, error) {
	return "", ErrNotIntegrated
}

func (unavailableService) CancelTransfer(string) (bool, error) {
	return false, ErrNotIntegrated
}
