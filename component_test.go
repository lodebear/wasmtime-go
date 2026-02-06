package wasmtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// newComponentEngine creates an engine with the component model enabled.
func newComponentEngine() *Engine {
	config := NewConfig()
	config.SetWasmComponentModel(true)
	return NewEngineWithConfig(config)
}

// newAsyncComponentEngine creates an engine with both component model and
// async component model enabled for testing async component features.
func newAsyncComponentEngine() *Engine {
	config := NewConfig()
	config.SetWasmComponentModel(true)
	config.SetAsyncSupport(true)
	config.SetWasmComponentModelAsync(true)
	config.SetWasmComponentModelErrorContext(true)
	return NewEngineWithConfig(config)
}

func TestComponentCompile(t *testing.T) {
	engine := newComponentEngine()

	// Compile a minimal component
	wasm, err := Wat2Wasm(`(component (core module))`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	// Invalid bytes should fail
	_, err = NewComponent(engine, []byte{})
	require.Error(t, err)
	_, err = NewComponent(engine, []byte{1, 2, 3})
	require.Error(t, err)
}

func TestComponentCompileFromWAT(t *testing.T) {
	engine := newComponentEngine()

	// Component with no imports or exports
	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()
}

func TestComponentSerializeDeserialize(t *testing.T) {
	engine := newComponentEngine()

	wasm, err := Wat2Wasm(`(component (core module))`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	// Serialize
	bytes, err := component.Serialize()
	require.NoError(t, err)
	require.NotEmpty(t, bytes)

	// Deserialize from bytes
	component2, err := NewComponentDeserialize(engine, bytes)
	require.NoError(t, err)
	defer component2.Close()

	// Deserialize from file
	tmpfile, err := os.CreateTemp("", "component-test")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.Write(bytes)
	require.NoError(t, err)
	tmpfile.Close()

	component3, err := NewComponentDeserializeFile(engine, tmpfile.Name())
	require.NoError(t, err)
	defer component3.Close()
}

func TestComponentLinkerInstantiate(t *testing.T) {
	engine := newComponentEngine()
	store := NewStore(engine)

	// Compile a minimal component (matches instantiate.cc test)
	wasm, err := Wat2Wasm(`(component (core module))`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}

func TestComponentLinkerInstantiateEmpty(t *testing.T) {
	engine := newComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}

func TestComponentLinkerAddWasip2(t *testing.T) {
	// Translates the wasip2.cc smoke test
	engine := newComponentEngine()
	store := NewStore(engine)

	// Set up WASI
	wasiConfig := NewWasiConfig()
	wasiConfig.InheritStdout()
	wasiConfig.InheritStderr()
	err := store.SetWasi(wasiConfig)
	require.NoError(t, err)

	// Component that imports wasi:cli/environment
	wasm, err := Wat2Wasm(`
		(component
		  (import "wasi:cli/environment@0.2.0" (instance
		    (export "get-arguments" (func (result (list string))))
		  ))
		)
	`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	// Add WASI p2 support
	err = linker.AddWasip2()
	require.NoError(t, err)

	// Instantiate should succeed with WASI imports satisfied
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}

func TestComponentLinkerAllowShadowing(t *testing.T) {
	// Translates the linker.cc allow_shadowing test
	engine := newComponentEngine()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	// Compile a core module to add via linker root
	coreWasm, err := Wat2Wasm(`(module)`)
	require.NoError(t, err)
	module, err := NewModule(engine, coreWasm)
	require.NoError(t, err)
	defer module.Close()

	// First definition should succeed
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddModule("x", module)
		require.NoError(t, err)
	})

	// Second definition with same name should fail (shadowing off by default)
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddModule("x", module)
		require.Error(t, err)
	})

	// Enable shadowing, then redefine should succeed
	linker.AllowShadowing(true)
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddModule("x", module)
		require.NoError(t, err)
	})
}

func TestComponentLinkerUnknownImportsAsTraps(t *testing.T) {
	// Translates the linker.cc unknown_imports_trap test
	engine := newComponentEngine()
	store := NewStore(engine)

	// Component that imports a function "a"
	wasm, err := Wat2Wasm(`
		(component
		  (import "a" (func))
		)
	`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	// Instantiation should fail because "a" is not defined
	_, err = linker.Instantiate(store, component)
	require.Error(t, err)

	// Define unknown imports as traps
	err = linker.DefineUnknownImportsAsTraps(component)
	require.NoError(t, err)

	// Now instantiation should succeed
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}

func TestComponentExportLookup(t *testing.T) {
	engine := newComponentEngine()

	// Component with an exported function
	wasm, err := Wat2Wasm(`
		(component
		  (core module $m
		    (func (export "f") (result i32) (i32.const 42))
		  )
		  (core instance $i (instantiate $m))
		  (func (export "f") (result u32)
		    (canon lift (core func $i "f"))
		  )
		)
	`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	// Look up export by name on the component
	exportIdx := component.GetExportIndex(nil, "f")
	require.NotNil(t, exportIdx, "expected to find export 'f'")
	defer exportIdx.Close()

	// Non-existent export should return nil
	missing := component.GetExportIndex(nil, "does-not-exist")
	require.Nil(t, missing, "expected nil for missing export")
}

func TestComponentInstanceGetFunc(t *testing.T) {
	engine := newComponentEngine()
	store := NewStore(engine)

	// Component with an exported function
	wasm, err := Wat2Wasm(`
		(component
		  (core module $m
		    (func (export "f") (result i32) (i32.const 42))
		  )
		  (core instance $i (instantiate $m))
		  (func (export "f") (result u32)
		    (canon lift (core func $i "f"))
		  )
		)
	`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)

	// Look up the export index on the instance
	exportIdx := instance.GetExportIndex(store, nil, "f")
	require.NotNil(t, exportIdx, "expected to find export 'f' on instance")
	defer exportIdx.Close()

	// Get the function from the instance via the export index
	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn, "expected to get a function for export 'f'")
}

func TestComponentInstanceGetFuncMissing(t *testing.T) {
	engine := newComponentEngine()
	store := NewStore(engine)

	// Component with no exports
	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// Non-existent export index should return nil
	exportIdx := instance.GetExportIndex(store, nil, "nope")
	require.Nil(t, exportIdx, "expected nil for missing export")
}

func TestComponentClose(t *testing.T) {
	engine := newComponentEngine()

	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)

	// Close should not panic
	component.Close()

	// Double close should not panic
	component.Close()

	// Using a closed component should panic
	require.Panics(t, func() {
		component.Serialize()
	})
}

func TestComponentLinkerClose(t *testing.T) {
	engine := newComponentEngine()

	linker := NewComponentLinker(engine)

	// Close should not panic
	linker.Close()

	// Double close should not panic
	linker.Close()

	// Using a closed linker should panic
	require.Panics(t, func() {
		linker.AllowShadowing(true)
	})
}

func TestComponentLinkerNestedInstance(t *testing.T) {
	engine := newComponentEngine()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	// Test nested instance creation through Root -> AddInstance
	linker.Root(func(root *ComponentLinkerInstance) {
		err := root.AddInstance("my-namespace", func(child *ComponentLinkerInstance) {
			// We can nest further
			err := child.AddInstance("inner", func(inner *ComponentLinkerInstance) {
				// inner exists; this is just a structure test
			})
			require.NoError(t, err)
		})
		require.NoError(t, err)
	})
}

func TestSetWasmComponentModel(t *testing.T) {
	// Verify that the config setting works - component compilation should fail
	// without the component model enabled, and succeed with it enabled.
	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)

	// Without component model enabled, compilation should fail
	cfgNo := NewConfig()
	cfgNo.SetWasmComponentModel(false)
	engineNoComponent := NewEngineWithConfig(cfgNo)
	_, err = NewComponent(engineNoComponent, wasm)
	require.Error(t, err)

	// With component model enabled, compilation should succeed
	engineWithComponent := newComponentEngine()
	component, err := NewComponent(engineWithComponent, wasm)
	require.NoError(t, err)
	defer component.Close()
}

func TestAsyncComponentEngineCreate(t *testing.T) {
	// Verify that creating an async engine doesn't panic
	engine := newAsyncComponentEngine()
	_ = engine

	// Verify we can compile a simple component with async engine
	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()
}

func TestAsyncCallSyncComponent(t *testing.T) {
	engine := newAsyncComponentEngine()
	store := NewStore(engine)

	// Component with add function: (u32, u32) -> u32
	wasm, err := Wat2Wasm(`
		(component
		  (core module $m
		    (func $add (export "add") (param i32 i32) (result i32)
		      local.get 0
		      local.get 1
		      i32.add
		    )
		    (memory (export "memory") 1)
		    ` + reallocAndFree + `
		  )
		  (core instance $i (instantiate $m))
		  (func (export "add") (param "a" u32) (param "b" u32) (result u32)
		    (canon lift (core func $i "add"))
		  )
		)
	`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.InstantiateAsync(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)

	// Look up the add function
	exportIdx := component.GetExportIndex(nil, "add")
	require.NotNil(t, exportIdx)
	defer exportIdx.Close()

	fn := instance.GetFunc(store, exportIdx)
	require.NotNil(t, fn)

	// Call it asynchronously
	results, err := fn.CallAsync(store, []ComponentVal{
		NewComponentValU32(34),
		NewComponentValU32(35),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, uint32(69), results[0].U32())
}

func TestAsyncInstantiateAsync(t *testing.T) {
	engine := newAsyncComponentEngine()
	store := NewStore(engine)

	wasm, err := Wat2Wasm(`(component)`)
	require.NoError(t, err)
	component, err := NewComponent(engine, wasm)
	require.NoError(t, err)
	defer component.Close()

	linker := NewComponentLinker(engine)
	defer linker.Close()

	instance, err := linker.InstantiateAsync(store, component)
	require.NoError(t, err)
	require.NotNil(t, instance)
}
