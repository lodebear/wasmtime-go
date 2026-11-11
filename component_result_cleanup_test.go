package wasmtime

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// Run with allocator poisoning set before libc starts. A failed component call
// leaves its C result slots uninitialized, so deleting those slots must never
// depend on the allocator returning zeroed or previously valid memory.
func TestComponentResultCleanup(t *testing.T) {
	if os.Getenv("WASMTIME_RESULT_CLEANUP_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestComponentResultCleanup$", "-test.v")
		cmd.Env = append(os.Environ(), "WASMTIME_RESULT_CLEANUP_CHILD=1", "MALLOC_PERTURB_=165", "GLIBC_TUNABLES=glibc.malloc.tcache_count=0")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}

	const wat = `(component
		(core module $m
			(memory (export "memory") 1)
			(data (i32.const 100) "hello world")
			(func (export "get") (result i32)
				(i32.store (i32.const 8) (i32.const 100))
				(i32.store offset=4 (i32.const 8) (i32.const 11))
				i32.const 8)
			(func (export "trap") (result i32) unreachable)
			(func (export "post-trap") (param i32) unreachable))
		(core instance $i (instantiate $m))
		(func (export "get") (result string)
			(canon lift (core func $i "get") (memory $i "memory")))
		(func (export "trap-string") (result string)
			(canon lift (core func $i "trap") (memory $i "memory")))
		(func (export "trap-list") (result (list u8))
			(canon lift (core func $i "trap") (memory $i "memory")))
		(func (export "post-trap") (result string)
			(canon lift (core func $i "get") (memory $i "memory")
				(post-return (func $i "post-trap")))))`

	for _, async := range []bool{false, true} {
		name := "sync"
		engine := newComponentEngine()
		if async {
			name = "async"
			engine = newAsyncComponentEngine()
		}
		t.Run(name, func(t *testing.T) {
			wasm, err := Wat2Wasm(wat)
			require.NoError(t, err)
			component, err := NewComponent(engine, wasm)
			require.NoError(t, err)
			defer component.Close()
			linker := NewComponentLinker(engine)
			defer linker.Close()

			for _, export := range []string{"get", "trap-string", "trap-list", "post-trap"} {
				t.Run(export, func(t *testing.T) {
					store := NewStore(engine)
					defer store.Close()
					var instance *ComponentInstance
					if async {
						instance, err = linker.InstantiateAsync(store, component)
					} else {
						instance, err = linker.Instantiate(store, component)
					}
					if async && err != nil {
						require.Contains(t, err.Error(), "component-linker-instantiate-async")
						// Older C API builds exercise CallAsync's nil-future error
						// path through a synchronously instantiated component.
						syncEngine := newComponentEngine()
						syncStore := NewStore(syncEngine)
						defer syncStore.Close()
						syncComponent, err := NewComponent(syncEngine, wasm)
						require.NoError(t, err)
						defer syncComponent.Close()
						syncLinker := NewComponentLinker(syncEngine)
						defer syncLinker.Close()
						syncInstance, err := syncLinker.Instantiate(syncStore, syncComponent)
						require.NoError(t, err)
						index := syncInstance.GetExportIndex(syncStore, nil, export)
						require.NotNil(t, index)
						defer index.Close()
						fn := syncInstance.GetFunc(syncStore, index)
						require.NotNil(t, fn)
						_, err = fn.CallAsync(syncStore, nil)
						require.Error(t, err)
						require.Contains(t, err.Error(), "component-func-call-async")
						return
					}
					require.NoError(t, err)
					index := instance.GetExportIndex(store, nil, export)
					require.NotNil(t, index)
					defer index.Close()
					fn := instance.GetFunc(store, index)
					require.NotNil(t, fn)
					var results []ComponentVal
					if async {
						results, err = fn.CallAsync(store, nil)
					} else {
						results, err = fn.Call(store, nil)
					}
					if export != "get" {
						require.Error(t, err)
						require.Contains(t, err.Error(), "unreachable")
						return
					}
					require.NoError(t, err)
					require.Len(t, results, 1)
					require.Equal(t, "hello world", results[0].String())
				})
			}
		})
	}
}
