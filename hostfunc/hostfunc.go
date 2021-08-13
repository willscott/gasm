package hostfunc

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/willscott/gasm/wasm"
)

type ModuleBuilder struct {
	modules map[string]*wasm.Module
}

func NewModuleBuilder() *ModuleBuilder {
	return &ModuleBuilder{modules: map[string]*wasm.Module{}}
}

func NewModuleBuilderWith(in map[string]*wasm.Module) *ModuleBuilder {
	return &ModuleBuilder{modules: in}
}

func (m *ModuleBuilder) Done() map[string]*wasm.Module {
	return m.modules
}

func (m *ModuleBuilder) MustSetFunction(modName, funcName string, fn func(machine *wasm.VirtualMachine) reflect.Value) {
	if err := m.SetFunction(modName, funcName, fn); err != nil {
		panic(err)
	}
}

func (m *ModuleBuilder) SetFunction(modName, funcName string, fn func(machine *wasm.VirtualMachine) reflect.Value) error {

	mod, ok := m.modules[modName]
	if !ok {
		mod = &wasm.Module{IndexSpace: new(wasm.ModuleIndexSpace), SecExports: map[string]*wasm.ExportSegment{}}
		m.modules[modName] = mod
	}

	mod.SecExports[funcName] = &wasm.ExportSegment{
		Name: funcName,
		Desc: &wasm.ExportDesc{
			Kind:  wasm.ExportKindFunction,
			Index: uint32(len(mod.IndexSpace.Function)),
		},
	}

	sig, err := getSignature(fn(&wasm.VirtualMachine{}).Type())
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	if len(sig.ReturnTypes) > 1 {
		// nope..
		sig.InputTypes = append([]wasm.ValueType{wasm.ValueTypeI32}, sig.InputTypes...)
		sig.ReturnTypes = []wasm.ValueType{}
		// decode that wasm peculiarity
		currFN := fn
		fn = func(vm *wasm.VirtualMachine) reflect.Value {
			actualFunc := currFN(vm)
			t := actualFunc.Type()
			ins := make([]reflect.Type, t.NumIn())
			for i := 0; i < t.NumIn(); i++ {
				ins[i] = t.In(i)
			}
			//wasm passes in a pointer to where the outputs should be written as the final parameter.
			ins = append([]reflect.Type{reflect.TypeOf(int32(0))}, ins...)
			outs := make([]reflect.Type, t.NumOut())
			for i := 0; i < t.NumOut(); i++ {
				outs[i] = t.Out(i)
			}
			newFuncSig := reflect.FuncOf(ins, []reflect.Type{}, false)
			return reflect.MakeFunc(newFuncSig, func(args []reflect.Value) []reflect.Value {
				funcArgs := args[1:]
				fmt.Printf("Calling %T\n", actualFunc.Interface())
				funcRet := actualFunc.Call(funcArgs)
				memloc := args[0].Int()
				fmt.Printf("interpreting 1st param as mem loc (it was: %d)\n", memloc)
				for i := len(outs) - 1; i >= 0; i-- {
					fmt.Printf("putting littlendian of %s: %v\n", outs[i].Kind().String(), funcRet[i].Interface())
					switch outs[i].Kind() {
					case reflect.Float64:
						binary.LittleEndian.PutUint64(vm.Memory[memloc:memloc+8], uint64(funcRet[i].Float()))
						memloc += 8
					case reflect.Float32:
						binary.LittleEndian.PutUint32(vm.Memory[memloc:memloc+4], uint32(float32(funcRet[i].Float())))
						memloc += 4
					case reflect.Int32:
						binary.LittleEndian.PutUint32(vm.Memory[memloc:memloc+4], uint32(funcRet[i].Int()))
						memloc += 4
					case reflect.Uint32:
						binary.LittleEndian.PutUint32(vm.Memory[memloc:memloc+4], uint32(funcRet[i].Uint()))
						memloc += 4
					case reflect.Int64:
						binary.LittleEndian.PutUint64(vm.Memory[memloc:memloc+8], uint64(funcRet[i].Int()))
						memloc += 8
					case reflect.Uint64:
						binary.LittleEndian.PutUint64(vm.Memory[memloc:memloc+8], funcRet[i].Uint())
						memloc += 8
					}
				}
				return []reflect.Value{}
			})
		}
	}

	mod.IndexSpace.Function = append(mod.IndexSpace.Function, &wasm.HostFunction{
		ClosureGenerator: fn,
		Signature:        sig,
	})
	return nil
}

func getSignature(p reflect.Type) (*wasm.FunctionType, error) {
	var err error
	in := make([]wasm.ValueType, p.NumIn())
	for i := range in {
		in[i], err = getTypeOf(p.In(i).Kind())
		if err != nil {
			return nil, err
		}
	}

	out := make([]wasm.ValueType, p.NumOut())
	for i := range out {
		out[i], err = getTypeOf(p.Out(i).Kind())
		if err != nil {
			return nil, err
		}
	}
	return &wasm.FunctionType{InputTypes: in, ReturnTypes: out}, nil
}

func getTypeOf(kind reflect.Kind) (wasm.ValueType, error) {
	switch kind {
	case reflect.Float64:
		return wasm.ValueTypeF64, nil
	case reflect.Float32:
		return wasm.ValueTypeF32, nil
	case reflect.Int32, reflect.Uint32:
		return wasm.ValueTypeI32, nil
	case reflect.Int64, reflect.Uint64:
		return wasm.ValueTypeI64, nil
	default:
		return 0x00, fmt.Errorf("invalid type: %s", kind.String())
	}
}
