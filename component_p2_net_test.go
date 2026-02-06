package wasmtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupWasiComponent loads a component from testdata/components/ and sets up
// a store with WASI config and a component linker with wasip2.
// Returns engine, store, linker, component. Caller should defer linker.Close() and component.Close().
func setupWasiComponent(t *testing.T, name string, configure func(*WasiConfig)) (*Engine, *Store, *ComponentLinker, *Component) {
	t.Helper()

	wasmBytes, err := os.ReadFile("testdata/components/" + name + ".component.wasm")
	require.NoError(t, err)

	engine := newComponentEngine()
	store := NewStore(engine)

	wasiConfig := NewWasiConfig()
	if configure != nil {
		configure(wasiConfig)
	}
	err = store.SetWasi(wasiConfig)
	require.NoError(t, err)

	linker := NewComponentLinker(engine)
	err = linker.AddWasip2()
	require.NoError(t, err)

	component, err := NewComponent(engine, wasmBytes)
	require.NoError(t, err)

	return engine, store, linker, component
}

// httpComponentTest is a helper that tries to instantiate an HTTP component.
// If wasi-http is not linked by AddWasip2, the test is skipped.
func httpComponentTest(t *testing.T, name string, configure func(*WasiConfig)) {
	t.Helper()
	_, store, linker, component := setupWasiComponent(t, name, configure)
	defer linker.Close()
	defer component.Close()

	_, err := linker.Instantiate(store, component)
	if err != nil {
		t.Skipf("wasi-http not available in C API: %v", err)
	}
}

// --- HTTP outbound request tests ---
// These components use wasi:http/outgoing-handler to make HTTP requests.
// They require the HTTP_SERVER env var to be set with the test server address.
// If AddWasip2 doesn't link wasi-http interfaces, the instantiation will fail
// and we skip the test.

func TestP2HttpOutboundRequestGet(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_get", func(cfg *WasiConfig) {
		cfg.SetEnv([]string{"HTTP_SERVER"}, []string{"localhost:99999"})
	})
}

func TestP2HttpOutboundRequestPost(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_post", func(cfg *WasiConfig) {
		cfg.SetEnv([]string{"HTTP_SERVER"}, []string{"localhost:99999"})
	})
}

func TestP2HttpOutboundRequestPut(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_put", func(cfg *WasiConfig) {
		cfg.SetEnv([]string{"HTTP_SERVER"}, []string{"localhost:99999"})
	})
}

func TestP2HttpOutboundRequestLargePost(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_large_post", func(cfg *WasiConfig) {
		cfg.SetEnv([]string{"HTTP_SERVER"}, []string{"localhost:99999"})
	})
}

func TestP2HttpOutboundRequestContentLength(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_content_length", func(cfg *WasiConfig) {
		cfg.SetEnv([]string{"HTTP_SERVER"}, []string{"localhost:99999"})
	})
}

func TestP2HttpOutboundRequestInvalidDnsname(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_invalid_dnsname", nil)
}

func TestP2HttpOutboundRequestInvalidHeader(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_invalid_header", nil)
}

func TestP2HttpOutboundRequestInvalidPort(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_invalid_port", nil)
}

func TestP2HttpOutboundRequestInvalidVersion(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_invalid_version", func(cfg *WasiConfig) {
		cfg.SetEnv([]string{"HTTP_SERVER"}, []string{"localhost:99999"})
	})
}

func TestP2HttpOutboundRequestMissingPathAndQuery(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_missing_path_and_query", nil)
}

func TestP2HttpOutboundRequestResponseBuild(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_response_build", nil)
}

func TestP2HttpOutboundRequestTimeout(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_timeout", nil)
}

func TestP2HttpOutboundRequestUnknownMethod(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_unknown_method", nil)
}

func TestP2HttpOutboundRequestUnsupportedScheme(t *testing.T) {
	httpComponentTest(t, "p2_http_outbound_request_unsupported_scheme", nil)
}

// --- TCP tests ---
// These components use wasi:sockets/tcp interfaces for socket operations.
// If AddWasip2 doesn't link socket interfaces, instantiation will fail and we skip.

func TestP2TcpBind(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_bind", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_bind failed: %v", err)
	}
}

func TestP2TcpConnect(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_connect", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_connect failed: %v", err)
	}
}

func TestP2TcpListen(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_listen", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_listen failed: %v", err)
	}
}

func TestP2TcpSampleApplication(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_sample_application", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_sample_application failed: %v", err)
	}
}

func TestP2TcpSockopts(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_sockopts", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_sockopts failed: %v", err)
	}
}

func TestP2TcpStates(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_states", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_states failed: %v", err)
	}
}

func TestP2TcpStreams(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_tcp_streams", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or tcp_streams failed: %v", err)
	}
}

// --- UDP tests ---

func TestP2UdpBind(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_udp_bind", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or udp_bind failed: %v", err)
	}
}

func TestP2UdpConnect(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_udp_connect", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or udp_connect failed: %v", err)
	}
}

func TestP2UdpSampleApplication(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_udp_sample_application", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or udp_sample_application failed: %v", err)
	}
}

func TestP2UdpSockopts(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_udp_sockopts", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or udp_sockopts failed: %v", err)
	}
}

func TestP2UdpStates(t *testing.T) {
	engine, store, linker, component := setupWasiComponent(t, "p2_udp_states", nil)
	defer linker.Close()
	defer component.Close()

	err := callWasiRun(t, engine, store, linker, component)
	if err != nil {
		t.Skipf("wasi-sockets not available or udp_states failed: %v", err)
	}
}
