//go:build !windows && !linux && !darwin

package service

type otherManager struct{}

func NewManager(cfg Config) (Manager, error) {
	return nil, ErrNotSupported
}
