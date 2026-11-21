//go:build !windows

package transfer

// NewEngineService fails closed outside supported Windows NTFS targets.
type unavailableEngineController struct{ unavailableService }

func NewEngineService(EngineServiceConfig) EngineController {
	return unavailableEngineController{}
}

func (unavailableEngineController) DecideReceive(string, string, bool) (bool, error) {
	return false, ErrNotIntegrated
}

func (unavailableEngineController) GetReceiveOffer(string) (ReceiveOffer, bool) {
	return ReceiveOffer{}, false
}

func (unavailableEngineController) GetSendCode(string) (string, bool) {
	return "", false
}
