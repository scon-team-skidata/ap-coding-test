package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

// TestSharedStateAcrossTransports verifies Gen1 HTTP, Gen2 HTTP RPC, and
// Gen2 WS RPC all read/write the same underlying relay state.
func TestSharedStateAcrossTransports(t *testing.T) {
	st := newState("test-device", false)
	mux := http.NewServeMux()
	registerGen1(mux, st)
	registerGen2HTTP(mux, st)
	registerWSRPC(mux, st)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Turn on via Gen1 REST.
	resp, err := http.Get(srv.URL + "/relay/0?turn=on")
	if err != nil {
		t.Fatalf("gen1 turn on: %v", err)
	}
	var gen1Status struct {
		Ison bool `json:"ison"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gen1Status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	if !gen1Status.Ison {
		t.Fatalf("expected relay on after gen1 turn=on")
	}

	// 2. Read via Gen2 HTTP RPC (/rpc/<Method> "params only" style, which
	// replies with the bare result per the Shelly HTTP RPC Channels spec)
	// and confirm it sees the same state.
	body, _ := json.Marshal(map[string]any{"id": 0})
	resp, err = http.Post(srv.URL+"/rpc/Switch.GetStatus", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gen2 get status: %v", err)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	if result["output"] != true {
		t.Fatalf("expected gen2 switch status output=true, got %v", result["output"])
	}

	// 3. Turn off via Gen2 WS RPC and confirm Gen1 sees it.
	wsURL := "ws" + srv.URL[len("http"):] + "/rpc"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close()

	params, _ := json.Marshal(map[string]any{"id": 0, "on": false})
	req := rpcRequest{ID: 1, Src: "test-client", Method: "Switch.Set", Params: params}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	var wsResp rpcResponse
	if err := conn.ReadJSON(&wsResp); err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if wsResp.Error != nil {
		t.Fatalf("ws rpc error: %+v", wsResp.Error)
	}

	resp, err = http.Get(srv.URL + "/relay/0")
	if err != nil {
		t.Fatalf("gen1 read: %v", err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&gen1Status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	if gen1Status.Ison {
		t.Fatalf("expected relay off after gen2 ws Switch.Set on=false")
	}
}

func TestBasicAuthRequired(t *testing.T) {
	cfg := defaultConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Username = "admin"
	cfg.Auth.Password = "secret"
	st := newState("test-device", cfg.Auth.Enabled)
	mux := http.NewServeMux()
	registerGen1(mux, st)

	srv := httptest.NewServer(withBasicAuth(cfg, mux))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/status")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got %d", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/status", nil)
	req.SetBasicAuth("admin", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authed request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with correct credentials, got %d", resp.StatusCode)
	}
}

func TestGen1AdvertisesAuthSetting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		mux := http.NewServeMux()
		registerGen1(mux, newState("test-device", enabled))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/shelly", nil))
		var info struct {
			Auth bool `json:"auth"`
		}
		if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
			t.Fatal(err)
		}
		if info.Auth != enabled {
			t.Errorf("auth=%v, want %v", info.Auth, enabled)
		}
	}
}

func TestHTTPSWithSelfSignedCert(t *testing.T) {
	cert, err := selfSignedCert()
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatalf("expected a generated certificate")
	}
}

// TestMultiDeviceIsolation verifies two devices configured via Config.Devices
// each get their own independent state (toggling one doesn't affect the other).
func TestMultiDeviceIsolation(t *testing.T) {
	cfg := defaultConfig()
	cfg.Devices = []DeviceConfig{
		{ID: "device-a", HTTPPort: 0},
		{ID: "device-b", HTTPPort: 0},
	}
	devs := cfg.devices()
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devs))
	}

	stA := newState(devs[0].ID, false)
	stB := newState(devs[1].ID, false)

	stA.SetRelay(true)

	onA, _, _, _ := stA.Snapshot()
	onB, _, _, _ := stB.Snapshot()
	if !onA {
		t.Fatalf("expected device-a relay on")
	}
	if onB {
		t.Fatalf("expected device-b relay to stay off, got on")
	}
}

// TestDevicesFallbackToSingle verifies that with no Devices list configured,
// a single device is derived from the top-level DeviceID/HTTPPort fields
// (back-compat with env-var based single-device config).
func TestDevicesFallbackToSingle(t *testing.T) {
	cfg := defaultConfig()
	devs := cfg.devices()
	if len(devs) != 1 || devs[0].ID != cfg.DeviceID || devs[0].HTTPPort != cfg.HTTPPort {
		t.Fatalf("expected single fallback device matching top-level config, got %+v", devs)
	}
}

// TestGen2HTTPEntireFrame verifies the "entire frame as payload" POST /rpc
// style per https://shelly-api-docs.shelly.cloud/gen2/General/RPCChannels,
// which replies with the full {id,src,result} envelope.
func TestGen2HTTPEntireFrame(t *testing.T) {
	st := newState("test-device", false)
	mux := http.NewServeMux()
	registerGen1(mux, st)
	registerGen2HTTP(mux, st)
	registerWSRPC(mux, st)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body, _ := json.Marshal(rpcRequest{ID: 7, Method: "Switch.Set", Params: json.RawMessage(`{"id":0,"on":true}`)})
	resp, err := http.Post(srv.URL+"/rpc", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post /rpc: %v", err)
	}
	defer resp.Body.Close()

	var out rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != 7 || out.Src != "test-device" {
		t.Fatalf("expected envelope id=7 src=test-device, got %+v", out)
	}
	result, ok := out.Result.(map[string]any)
	if !ok || result["was_on"] != false {
		t.Fatalf("expected result.was_on=false, got %+v", out.Result)
	}
}

// TestGen2HTTPQueryString verifies the GET /rpc/<Method>?k=v scalar query
// style per the same RPCChannels doc.
func TestGen2HTTPQueryString(t *testing.T) {
	st := newState("test-device", false)
	mux := http.NewServeMux()
	registerGen1(mux, st)
	registerGen2HTTP(mux, st)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/rpc/Switch.Set?id=0&on=true")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["was_on"] != false {
		t.Fatalf("expected bare result was_on=false, got %+v", result)
	}

	on, _, _, _ := st.Snapshot()
	if !on {
		t.Fatalf("expected relay on after GET query-string Switch.Set")
	}
}
