package wasmtime

import "testing"

// Specialty component tests — each requires specific runtime infrastructure
// not available in the standard Wasmtime C API.

// DWARF debug info tests
func TestDwarfMultipleCodegenUnits(t *testing.T) {
	t.Skip("DWARF debug info test requires debugger integration not available in Go bindings")
}

func TestDwarfSimple(t *testing.T) {
	t.Skip("DWARF debug info test requires debugger integration not available in Go bindings")
}

// Fuzzing tests
func TestFuzzAsync(t *testing.T) {
	t.Skip("fuzzing test, not applicable to Go bindings")
}

// wasi-nn tests
func TestNnWitImageClassificationOnnx(t *testing.T) {
	t.Skip("requires wasi-nn runtime")
}

func TestNnWitImageClassificationOpenvinoNamed(t *testing.T) {
	t.Skip("requires wasi-nn runtime")
}

func TestNnWitImageClassificationOpenvino(t *testing.T) {
	t.Skip("requires wasi-nn runtime")
}

func TestNnWitImageClassificationPytorch(t *testing.T) {
	t.Skip("requires wasi-nn runtime")
}

func TestNnWitImageClassificationWinmlNamed(t *testing.T) {
	t.Skip("requires wasi-nn runtime")
}

// Piped I/O tests — these test stdin/stdout piping between components,
// which requires async component model support for the P3-era stream types.
func TestPipedMultiple(t *testing.T) {
	t.Skip("piped I/O test requires async component model support not available in C API v41")
}

func TestPipedPolling(t *testing.T) {
	t.Skip("piped I/O test requires async component model support not available in C API v41")
}

func TestPipedSimple(t *testing.T) {
	t.Skip("piped I/O test requires async component model support not available in C API v41")
}

// wasi-config test
func TestConfigGet(t *testing.T) {
	t.Skip("requires wasi-config interface")
}

// wasi-keyvalue test
func TestKeyvalueMain(t *testing.T) {
	t.Skip("requires wasi-keyvalue interface")
}

// wasi-tls test
func TestTlsSampleApplication(t *testing.T) {
	t.Skip("requires wasi-tls infrastructure")
}
