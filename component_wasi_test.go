package wasmtime

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// callWasiRun instantiates a WASI component and calls its wasi:cli/run@0.2.0#run export.
// Returns an error if the run function returns an error (e.g. non-zero exit code).
func callWasiRun(t *testing.T, engine *Engine, store *Store, linker *ComponentLinker, component *Component) error {
	t.Helper()

	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	// Look up the "wasi:cli/run@0.2.0" instance export first
	runInstanceIdx := instance.GetExportIndex(store, nil, "wasi:cli/run@0.2.0")
	if runInstanceIdx == nil {
		// Some components may not export the run interface — just instantiation runs them
		return nil
	}
	defer runInstanceIdx.Close()

	// Then look up "run" within that instance
	runFuncIdx := instance.GetExportIndex(store, runInstanceIdx, "run")
	if runFuncIdx == nil {
		return nil
	}
	defer runFuncIdx.Close()

	fn := instance.GetFunc(store, runFuncIdx)
	if fn == nil {
		return nil
	}

	results, err := fn.Call(store, nil)
	if err != nil {
		return err
	}

	// wasi:cli/run#run returns result<_, _> — check if it's an error
	if len(results) > 0 {
		r := results[0].Result()
		if !r.IsOk {
			return errors.New("run returned error result")
		}
	}

	return nil
}

func TestWASICliStdout(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/cli-stdout.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "hello from wasi")
}

func TestWASICliStderr(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/cli-stderr.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stderrFile := filepath.Join(tmpDir, "stderr.txt")

	wasiConfig := NewWasiConfig()
	err = wasiConfig.SetStderrFile(stderrFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stderr, err := os.ReadFile(stderrFile)
	require.NoError(t, err)
	require.Contains(t, string(stderr), "error message")
}

func TestWASICliArgs(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/cli-args.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"arg1", "arg2", "arg3"})
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	output := string(stdout)
	require.Contains(t, output, "arg1")
	require.Contains(t, output, "arg2")
	require.Contains(t, output, "arg3")
}

func TestWASICliEnv(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/cli-env.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	wasiConfig.SetEnv([]string{"TEST_KEY"}, []string{"test_value"})
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "test_value")
}

func TestWASICliStdinEcho(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/cli-stdin-echo.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdinFile := filepath.Join(tmpDir, "stdin.txt")
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	err = os.WriteFile(stdinFile, []byte("hello stdin"), 0644)
	require.NoError(t, err)

	wasiConfig := NewWasiConfig()
	err = wasiConfig.SetStdinFile(stdinFile)
	require.NoError(t, err)
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "hello stdin")
}

func TestWASIFsRead(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/fs-read.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	// Create a test file in the preopened directory
	testContent := "file content from host"
	testFile := filepath.Join(tmpDir, "input.txt")
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"test", "/testdir/input.txt"})
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = wasiConfig.PreopenDir(tmpDir, "/testdir", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	require.Contains(t, string(stdout), testContent)
}

func TestWASIFsWrite(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/fs-write.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"test", "/testdir/output.txt"})
	err = wasiConfig.PreopenDir(tmpDir, "/testdir", DIR_READ|DIR_WRITE, FILE_READ|FILE_WRITE)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	require.Contains(t, string(content), "written by wasi")
}

func TestWASIClockNow(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/clock-now.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	output := strings.TrimSpace(string(stdout))
	require.NotEmpty(t, output, "expected non-empty clock output")

	// The component outputs "mono:<N> wall:<N>" — parse both timestamps
	require.Contains(t, output, "wall:")
	for _, field := range strings.Fields(output) {
		parts := strings.SplitN(field, ":", 2)
		require.Len(t, parts, 2, "expected key:value format, got: %s", field)
		val, err := strconv.ParseUint(parts[1], 10, 64)
		require.NoError(t, err, "expected numeric value in %s", field)
		if parts[0] == "wall" {
			require.NotZero(t, val, "expected non-zero wall clock timestamp")
		}
	}
}

func TestWASIRandomBytes(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/random-bytes.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	tmpDir := t.TempDir()
	stdoutFile := filepath.Join(tmpDir, "stdout.txt")

	wasiConfig := NewWasiConfig()
	err = wasiConfig.SetStdoutFile(stdoutFile)
	require.NoError(t, err)
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	err = callWasiRun(t, engine, store, linker, component)
	require.NoError(t, err)

	stdout, err := os.ReadFile(stdoutFile)
	require.NoError(t, err)
	output := strings.TrimSpace(string(stdout))

	// Expect 32 hex chars (16 bytes)
	require.Len(t, output, 32, "expected 32 hex characters for 16 random bytes, got: %s", output)
	decoded, err := hex.DecodeString(output)
	require.NoError(t, err, "expected valid hex output, got: %s", output)
	require.Len(t, decoded, 16)

	// Verify not all zeros (astronomically unlikely for real random)
	allZero := true
	for _, b := range decoded {
		if b != 0 {
			allZero = false
			break
		}
	}
	require.False(t, allZero, "random bytes should not all be zero")
}

func TestWASIExitCode(t *testing.T) {
	wasmBytes, err := os.ReadFile("testdata/exit-code.component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	wasiConfig := NewWasiConfig()
	wasiConfig.SetArgv([]string{"test", "1"})
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	defer linker.Close()
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)
	defer component.Close()

	// A non-zero exit code should propagate as an error
	instance, err := linker.Instantiate(store, component)
	require.NoError(t, err)

	runInstanceIdx := instance.GetExportIndex(store, nil, "wasi:cli/run@0.2.0")
	require.NotNil(t, runInstanceIdx, "expected wasi:cli/run@0.2.0 export")
	defer runInstanceIdx.Close()

	runFuncIdx := instance.GetExportIndex(store, runInstanceIdx, "run")
	require.NotNil(t, runFuncIdx, "expected run function export")
	defer runFuncIdx.Close()

	fn := instance.GetFunc(store, runFuncIdx)
	require.NotNil(t, fn, "expected to get run function")

	_, err = fn.Call(store, nil)
	require.Error(t, err, "expected error for non-zero exit code")
}
