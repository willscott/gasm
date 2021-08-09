package wasm

import (
	"errors"
	"fmt"
	"io"

	"github.com/mathetake/gasm/wasm/leb128"
)

type SectionID byte

const (
	SectionIDCustom   SectionID = 0
	SectionIDType     SectionID = 1
	SectionIDImport   SectionID = 2
	SectionIDFunction SectionID = 3
	SectionIDTable    SectionID = 4
	SectionIDMemory   SectionID = 5
	SectionIDGlobal   SectionID = 6
	SectionIDExport   SectionID = 7
	SectionIDStart    SectionID = 8
	SectionIDElement  SectionID = 9
	SectionIDCode     SectionID = 10
	SectionIDData     SectionID = 11
)

func (m *Module) readSections(r io.Reader, gas GasMeter) error {
	for {
		if err := m.readSection(r, gas); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

func (m *Module) readSection(r io.Reader, gas GasMeter) error {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return fmt.Errorf("read section id: %w", err)
	}
	gas.Step(1)

	ss, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of section for id=%d: %w", SectionID(b[0]), err)
	}
	gas.Step(4)

	switch SectionID(b[0]) {
	case SectionIDCustom:
		// TODO: should support custom section
		bb := make([]byte, ss)
		_, err = io.ReadFull(r, bb)
		gas.Step(int64(ss))
	case SectionIDType:
		err = m.readSectionTypes(r, gas)
	case SectionIDImport:
		err = m.readSectionImports(r, gas)
	case SectionIDFunction:
		err = m.readSectionFunctions(r, gas)
	case SectionIDTable:
		err = m.readSectionTables(r, gas)
	case SectionIDMemory:
		err = m.readSectionMemories(r, gas)
	case SectionIDGlobal:
		err = m.readSectionGlobals(r, gas)
	case SectionIDExport:
		err = m.readSectionExports(r, gas)
	case SectionIDStart:
		err = m.readSectionStart(r, gas)
	case SectionIDElement:
		err = m.readSectionElement(r, gas)
	case SectionIDCode:
		err = m.readSectionCodes(r, gas)
	case SectionIDData:
		err = m.readSectionData(r, gas)
	default:
		err = errors.New("invalid section id")
	}

	if err != nil {
		return fmt.Errorf("read section for %d: %w", SectionID(b[0]), err)
	}
	return nil
}

func (m *Module) readSectionTypes(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}

	m.SecTypes = make([]*FunctionType, vs)
	for i := range m.SecTypes {
		m.SecTypes[i], err = readFunctionType(r, gas)
		if err != nil {
			return fmt.Errorf("read %d-th function type: %w", i, err)
		}
	}
	return nil
}

func (m *Module) readSectionImports(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecImports = make([]*ImportSegment, vs)
	for i := range m.SecImports {
		m.SecImports[i], err = readImportSegment(r, gas)
		if err != nil {
			return fmt.Errorf("read import: %w", err)
		}
	}
	return nil
}

func (m *Module) readSectionFunctions(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecFunctions = make([]uint32, vs)
	for i := range m.SecFunctions {
		m.SecFunctions[i], _, err = leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("get typeidx: %w", err)
		}
		gas.Step(4)
	}
	return nil
}

func (m *Module) readSectionTables(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecTables = make([]*TableType, vs)
	for i := range m.SecTables {
		m.SecTables[i], err = readTableType(r, gas)
		if err != nil {
			return fmt.Errorf("read table type: %w", err)
		}
	}
	return nil
}

func (m *Module) readSectionMemories(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecMemory = make([]*MemoryType, vs)
	for i := range m.SecMemory {
		m.SecMemory[i], err = readMemoryType(r, gas)
		if err != nil {
			return fmt.Errorf("read memory type: %w", err)
		}
	}
	return nil
}

func (m *Module) readSectionGlobals(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecGlobals = make([]*GlobalSegment, vs)
	for i := range m.SecGlobals {
		m.SecGlobals[i], err = readGlobalSegment(r, gas)
		if err != nil {
			return fmt.Errorf("read global segment: %w ", err)
		}
	}
	return nil
}

func (m *Module) readSectionExports(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecExports = make(map[string]*ExportSegment, vs)
	for i := uint32(0); i < vs; i++ {
		expDesc, err := readExportSegment(r, gas)
		if err != nil {
			return fmt.Errorf("read export: %w", err)
		}

		m.SecExports[expDesc.Name] = expDesc
	}
	return nil
}

func (m *Module) readSectionStart(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecStart = make([]uint32, vs)
	for i := range m.SecStart {
		m.SecStart[i], _, err = leb128.DecodeUint32(r)
		if err != nil {
			return fmt.Errorf("read function index: %w", err)
		}
		gas.Step(4)
	}
	return nil
}

func (m *Module) readSectionElement(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecElements = make([]*ElementSegment, vs)
	for i := range m.SecElements {
		m.SecElements[i], err = readElementSegment(r, gas)
		if err != nil {
			return fmt.Errorf("read element: %w", err)
		}
	}
	return nil
}

func (m *Module) readSectionCodes(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	m.SecCodes = make([]*CodeSegment, vs)
	gas.Step(4)

	for i := range m.SecCodes {
		m.SecCodes[i], err = readCodeSegment(r, gas)
		if err != nil {
			return fmt.Errorf("read code segment: %w", err)
		}
	}
	return nil
}

func (m *Module) readSectionData(r io.Reader, gas GasMeter) error {
	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return fmt.Errorf("get size of vector: %w", err)
	}
	gas.Step(4)

	m.SecData = make([]*DataSegment, vs)
	for i := range m.SecData {
		m.SecData[i], err = readDataSegment(r, gas)
		if err != nil {
			return fmt.Errorf("read data segment: %w", err)
		}
	}
	return nil
}
