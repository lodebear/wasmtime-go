# WASI P2 Component Test Validation Report

**Verifier**: verifier agent
**Date**: 2026-02-06
**Verdict**: **PASS**

---

## 1. Authenticity Checks

| Check | Result |
|-------|--------|
| Every test loads .wasm via `os.ReadFile` | PASS - all 10 tests use `os.ReadFile("testdata/<name>.component.wasm")` |
| Zero `Wat2Wasm` or WAT literals in component_wasi_test.go | PASS - grep returns 0 matches |
| Zero `(component`, `(core module`, `(module` in component_wasi_test.go | PASS - grep returns 0 matches |
| All .wasm files are real WebAssembly binaries | PASS - `file` reports "WebAssembly (wasm) binary module version 0x1000d" (component model) |
| Magic bytes correct | PASS - all start with `00 61 73 6d 0d 00 01 00` (\0asm + component version) |
| File sizes consistent with real Rust WASI P2 components | PASS - range 82-130 KB (WAT stubs would be <5 KB) |
| Rust source code exists and does real WASI I/O | PASS - 10 `main.rs` files in `testdata/wasi-components/` |
| Built with `cargo-component` 0.21.1 | PASS - confirmed via process listing during build |

## 2. I/O Verification Checks

| WASI Interface | Test Name | Rust Code | Go Assertion | Result |
|----------------|-----------|-----------|--------------|--------|
| **stdout** | `TestWASICliStdout` | `print!("hello from wasi")` | `Contains("hello from wasi")` | PASS |
| **stderr** | `TestWASICliStderr` | `eprint!("error message")` | `Contains("error message")` | PASS |
| **stdin** | `TestWASICliStdinEcho` | `stdin().read_to_string()` → stdout | Writes "hello stdin" to file, checks `Contains("hello stdin")` | PASS |
| **CLI args** | `TestWASICliArgs` | `env::args().collect()` → stdout | `Contains("arg1")`, `Contains("arg2")`, `Contains("arg3")` | PASS |
| **Env vars** | `TestWASICliEnv` | `env::var("TEST_KEY")` → stdout | Sets `TEST_KEY=test_value`, checks `Contains("test_value")` | PASS |
| **Filesystem read** | `TestWASIFsRead` | `fs::read_to_string(path)` → stdout | Creates file via host, preopens dir, checks `Contains("file content from host")` | PASS |
| **Filesystem write** | `TestWASIFsWrite` | `fs::write(path, "written by wasi")` | Preopens dir, reads written file, checks `Contains("written by wasi")` | PASS |
| **Clocks** | `TestWASIClockNow` | `SystemTime::now()` + `Instant::now()` | Parses `mono:<N> wall:<N>`, verifies wall clock > 0 | PASS |
| **Random** | `TestWASIRandomBytes` | `getrandom::getrandom(&mut [0u8; 16])` → hex | 32 hex chars, decodes to 16 bytes, not all zeros | PASS |
| **Exit code** | `TestWASIExitCode` | `process::exit(1)` | `require.Error` on fn.Call | PASS |

## 3. Negative Checks

| Check | Result |
|-------|--------|
| Non-zero exit code propagates as error | PASS - `TestWASIExitCode` calls with exit code 1, asserts error |
| Tests verify SPECIFIC expected strings (not just "no error") | PASS - every test checks specific content |
| Explicit "no AddWasip2() causes failure" test | NOT PRESENT (minor gap) |

**Note on missing negative test**: While there is no explicit test that omits `AddWasip2()` and asserts `Instantiate` fails, all 10 tests call `AddWasip2()` and would fail at instantiation if it were removed. The existing `TestComponentLinkerAddWasip2` in `component_test.go` also exercises the linker's WASI integration (though via WAT, which is appropriate for that test).

## 4. Code Quality Checks

| Check | Result |
|-------|--------|
| `go build ./...` | PASS - clean |
| `go vet ./...` | PASS - clean |
| `go test -count=1 ./...` | PASS - all tests pass in 1.630s |
| No weakened assertions | PASS - all assertions check specific values |
| Fixes in implementation, not tests | PASS - `Store.SetWasi` was fixed to return error (correct C API binding) |
| No regressions in existing tests | PASS - full test suite passes |

## 5. Implementation Fix Review

The fixer made one change to `store.go`:

```go
// Before (incorrect - ignoring C API error):
func (store *Store) SetWasi(wasi *WasiConfig) {
    C.wasmtime_context_set_wasi(store.Context(), ptr)
}

// After (correct - propagating C API error):
func (store *Store) SetWasi(wasi *WasiConfig) error {
    err := C.wasmtime_context_set_wasi(store.Context(), ptr)
    if err != nil { return mkError(err) }
    return nil
}
```

This is a legitimate fix. The C API `wasmtime_context_set_wasi` returns `wasmtime_error_t*` and the old code silently ignored it. The `example_test.go` was also updated to handle the new return type.

## 6. Component Source Audit

All 10 Rust components are minimal, focused programs that exercise exactly one WASI interface each:

- **cli-stdout**: 3 lines, uses `print!` (stdout)
- **cli-stderr**: 3 lines, uses `eprint!` (stderr)
- **cli-args**: 4 lines, uses `std::env::args` (arguments)
- **cli-env**: 5 lines, uses `std::env::var` (environment)
- **cli-stdin-echo**: 6 lines, uses `std::io::stdin` (stdin)
- **fs-read**: 4 lines, uses `std::fs::read_to_string` (filesystem read)
- **fs-write**: 4 lines, uses `std::fs::write` (filesystem write)
- **clock-now**: 12 lines, uses `SystemTime::now()` and `Instant::now()` (clocks)
- **random-bytes**: 5 lines, uses `getrandom::getrandom` (random)
- **exit-code**: 7 lines, uses `std::process::exit` (exit code)

No component is doing pure computation without WASI I/O. Every component exercises a real WASI interface.

## 7. Summary

All 10 WASI P2 component tests use **real compiled Rust WASI components** (built with `cargo-component` 0.21.1). Every component exercises a genuine WASI interface (stdout, stderr, stdin, args, env, filesystem read/write, clocks, random, exit). Tests verify **specific expected output** — not just "no error". The implementation was fixed correctly (not the tests weakened). The full test suite passes.

**We can honestly say: real Rust WASI P2 components run through our Go host.**
