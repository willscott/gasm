package wasm

import "errors"

var (
	ErrOutOfGas = errors.New("out of gas")
)

// GasMeter is used for tracking gas consumption of execution.
// the fee schedule is based on
// https://ewasm.readthedocs.io/en/mkdocs/fee_schedule/
type GasMeter interface {
	Step(int64)
	Exceeded() bool
}

type gas struct {
	remaining int64
}

// NewGas creates a gas meter with the specified amount of gas
func NewGas(limit int64) GasMeter {
	return &gas{limit}
}

func (g *gas) Step(n int64) {
	g.remaining -= n
	if g.remaining <= 0 {
		panic(ErrOutOfGas)
	}
}

func (g *gas) Exceeded() bool {
	return g.remaining <= 0
}

type unmetered struct {
}

func (u *unmetered) Step(n int64) {
	return
}

func (u *unmetered) Exceeded() bool {
	return false
}
