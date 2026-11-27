package dispatch

import "errors"

var ErrDispatchCircuitOpen = errors.New("dispatch circuit open")

// DispatchCircuitBreaker trips after maxFail consecutive dispatch failures.
type DispatchCircuitBreaker struct {
	maxFail int
	fails   int
	open    bool
}

func NewDispatchCircuitBreaker(maxFail int) *DispatchCircuitBreaker {
	return &DispatchCircuitBreaker{maxFail: maxFail}
}

func (cb *DispatchCircuitBreaker) Call(fn func() error) error {
	if cb.open {
		return ErrDispatchCircuitOpen
	}
	err := fn()
	if err != nil {
		cb.fails++
		if cb.fails >= cb.maxFail {
			cb.open = true
		}
		return err
	}
	cb.fails = 0
	return nil
}
