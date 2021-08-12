package wasm

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/willscott/gasm/wasm/leb128"
)

type ImportDesc struct {
	Kind byte

	TypeIndexPtr  *uint32
	TableTypePtr  *TableType
	MemTypePtr    *MemoryType
	GlobalTypePtr *GlobalType
}

func readImportDesc(r io.Reader, gas GasMeter) (*ImportDesc, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read value kind: %w", err)
	}
	gas.Step(1)

	switch b[0] {
	case 0x00:
		tID, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read typeindex: %w", err)
		}
		gas.Step(4)
		return &ImportDesc{
			Kind:         0x00,
			TypeIndexPtr: &tID,
		}, nil
	case 0x01:
		tt, err := readTableType(r, gas)
		if err != nil {
			return nil, fmt.Errorf("read table type: %w", err)
		}
		return &ImportDesc{
			Kind:         0x01,
			TableTypePtr: tt,
		}, nil
	case 0x02:
		mt, err := readMemoryType(r, gas)
		if err != nil {
			return nil, fmt.Errorf("read table type: %w", err)
		}
		return &ImportDesc{
			Kind:       0x02,
			MemTypePtr: mt,
		}, nil
	case 0x03:
		gt, err := readGlobalType(r, gas)
		if err != nil {
			return nil, fmt.Errorf("read global type: %w", err)
		}

		return &ImportDesc{
			Kind:          0x03,
			GlobalTypePtr: gt,
		}, nil
	default:
		return nil, fmt.Errorf("%w: invalid byte for importdesc: %#x", ErrInvalidByte, b[0])
	}
}

type ImportSegment struct {
	Module, Name string
	Desc         *ImportDesc
}

func readImportSegment(r io.Reader, gas GasMeter) (*ImportSegment, error) {
	mn, err := readNameValue(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read name of imported module: %w", err)
	}

	n, err := readNameValue(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read name of imported module component: %w", err)
	}

	d, err := readImportDesc(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read import description : %w", err)
	}

	return &ImportSegment{Module: mn, Name: n, Desc: d}, nil
}

type GlobalSegment struct {
	Type *GlobalType
	Init *ConstantExpression
}

func readGlobalSegment(r io.Reader, gas GasMeter) (*GlobalSegment, error) {
	gt, err := readGlobalType(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read global type: %w", err)
	}

	init, err := readConstantExpression(r, gas)
	if err != nil {
		return nil, fmt.Errorf("get init expression: %w", err)
	}

	return &GlobalSegment{
		Type: gt,
		Init: init,
	}, nil
}

type ExportDesc struct {
	Kind  byte
	Index uint32
}

const (
	ExportKindFunction byte = 0x00
	ExportKindTable    byte = 0x01
	ExportKindMem      byte = 0x02
	ExportKindGlobal   byte = 0x03
)

func readExportDesc(r io.Reader, gas GasMeter) (*ExportDesc, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read value kind: %w", err)
	}
	gas.Step(1)

	kind := b[0]
	if kind >= 0x04 {
		return nil, fmt.Errorf("%w: invalid byte for exportdesc: %#x", ErrInvalidByte, kind)
	}

	id, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read funcidx: %w", err)
	}
	gas.Step(4)

	return &ExportDesc{
		Kind:  kind,
		Index: id,
	}, nil

}

type ExportSegment struct {
	Name string
	Desc *ExportDesc
}

func readExportSegment(r io.Reader, gas GasMeter) (*ExportSegment, error) {
	name, err := readNameValue(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read name of export module: %w", err)
	}

	d, err := readExportDesc(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read export description: %w", err)
	}

	return &ExportSegment{Name: name, Desc: d}, nil
}

type ElementSegment struct {
	TableIndex uint32
	OffsetExpr *ConstantExpression
	Init       []uint32
}

func readElementSegment(r io.Reader, gas GasMeter) (*ElementSegment, error) {
	ti, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get table index: %w", err)
	}
	gas.Step(4)

	expr, err := readConstantExpression(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read expr for offset: %w", err)
	}

	if expr.optCode != OptCodeI32Const {
		return nil, fmt.Errorf("offset expression must be i32.const but go %#x", expr.optCode)
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	init := make([]uint32, vs)
	for i := range init {
		fIDx, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read function index: %w", err)
		}
		gas.Step(4)
		init[i] = fIDx
	}

	return &ElementSegment{
		TableIndex: ti,
		OffsetExpr: expr,
		Init:       init,
	}, nil
}

type CodeSegment struct {
	NumLocals uint32
	Body      []byte
}

func readCodeSegment(r io.Reader, gas GasMeter) (*CodeSegment, error) {
	ss, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of code segment: %w", err)
	}
	gas.Step(4)

	r = io.LimitReader(r, int64(ss))

	// parse locals
	ls, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size locals: %w", err)
	}

	var numLocals uint32
	b := make([]byte, 1)
	for i := uint32(0); i < ls; i++ {
		n, _, err := leb128.DecodeUint32(r)
		if err != nil {
			return nil, fmt.Errorf("read n of locals: %w", err)
		}
		numLocals += n

		if _, err := io.ReadFull(r, b); err != nil {
			return nil, fmt.Errorf("read type of local")
		}
	}

	// extract body
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if body[len(body)-1] != byte(OptCodeEnd) {
		return nil, fmt.Errorf("expr not end with OptCodeEnd")
	}
	gas.Step(int64(ss))

	return &CodeSegment{
		Body:      body[:len(body)-1],
		NumLocals: numLocals,
	}, nil
}

type DataSegment struct {
	MemoryIndex      uint32 // supposed to be zero
	OffsetExpression *ConstantExpression
	Init             []byte
}

func readDataSegment(r io.Reader, gas GasMeter) (*DataSegment, error) {
	d, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read memory index: %w", err)
	}
	gas.Step(4)

	if d != 0 {
		return nil, fmt.Errorf("invalid memory index: %d", d)
	}

	expr, err := readConstantExpression(r, gas)
	if err != nil {
		return nil, fmt.Errorf("read offset expression: %w", err)
	}

	if expr.optCode != OptCodeI32Const {
		return nil, fmt.Errorf("offset expression must have i32.const optcode but go %#x", expr.optCode)
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of vector: %w", err)
	}
	gas.Step(4)

	b := make([]byte, vs)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read bytes for init: %w", err)
	}
	gas.Step(int64(vs))

	return &DataSegment{
		OffsetExpression: expr,
		Init:             b,
	}, nil
}
