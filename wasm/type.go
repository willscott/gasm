package wasm

import (
	"errors"
	"fmt"
	"io"

	"github.com/willscott/gasm/wasm/leb128"
)

var (
	ErrInvalidByte = errors.New("invalid byte")
)

type FunctionType struct {
	InputTypes, ReturnTypes []ValueType
}

func readFunctionType(r io.Reader, gas GasMeter) (*FunctionType, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}
	gas.Step(1)

	if b[0] != 0x60 {
		return nil, fmt.Errorf("%w: %#x != 0x60", ErrInvalidByte, b[0])
	}

	s, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of input value types: %w", err)
	}
	gas.Step(4)

	ip, err := readValueTypes(r, s, gas)
	if err != nil {
		return nil, fmt.Errorf("read value types of inputs: %w", err)
	}

	s, _, err = leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of output value types: %w", err)
	}
	gas.Step(4)

	op, err := readValueTypes(r, s, gas)
	if err != nil {
		return nil, fmt.Errorf("read value types of outputs: %w", err)
	}

	return &FunctionType{
		InputTypes:  ip,
		ReturnTypes: op,
	}, nil
}

type LimitsType struct {
	Min uint32
	Max *uint32
}

func readLimitsType(r io.Reader, gas GasMeter) (*LimitsType, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}
	gas.Step(1)

	ret := &LimitsType{}
	switch b[0] {
	case 0x00:
		ret.Min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		gas.Step(4)
	case 0x01:
		ret.Min, _, err = leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		m, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read min of limit: %w", err)
		}
		gas.Step(8)
		ret.Max = &m
	default:
		return nil, fmt.Errorf("%w for limits: %#x != 0x00 or 0x01", ErrInvalidByte, b[0])
	}
	return ret, nil
}

type TableType struct {
	Elem  byte
	Limit *LimitsType
}

func readTableType(r io.Reader, gas GasMeter) (*TableType, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read leading byte: %w", err)
	}
	gas.Step(1)

	if b[0] != 0x70 {
		return nil, fmt.Errorf("%w: invalid element type %#x != %#x", ErrInvalidByte, b[0], 0x70)
	}

	lm, err := readLimitsType(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read limits: %w", err)
	}

	return &TableType{
		Elem:  0x70,
		Limit: lm,
	}, nil
}

type MemoryType = LimitsType

func readMemoryType(r io.Reader, gas GasMeter) (*MemoryType, error) {
	return readLimitsType(r, gas)
}

type GlobalType struct {
	Value   ValueType
	Mutable bool
}

func readGlobalType(r io.Reader, gas GasMeter) (*GlobalType, error) {
	vt, err := readValueTypes(r, 1, gas)
	if err != nil {
		return nil, fmt.Errorf("read value type: %w", err)
	}

	ret := &GlobalType{
		Value: vt[0],
	}

	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read mutablity: %w", err)
	}
	gas.Step(1)

	switch mut := b[0]; mut {
	case 0x00:
	case 0x01:
		ret.Mutable = true
	default:
		return nil, fmt.Errorf("%w for mutability: %#x != 0x00 or 0x01", ErrInvalidByte, mut)
	}
	return ret, nil
}
