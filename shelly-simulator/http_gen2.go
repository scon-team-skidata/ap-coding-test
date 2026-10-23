package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// rpcRequest/rpcResponse match the Shelly Gen2 JSON-RPC envelope used
// over HTTP, WebSocket, and MQTT alike. Field names/shapes follow
// https://shelly-api-docs.shelly.cloud/gen2/General/RPCProtocol
type rpcRequest struct {
	ID     int             `json:"id,omitempty"`
	Src    string          `json:"src,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     int     `json:"id,omitempty"`
	Src    string  `json:"src,omitempty"`
	Dst    string  `json:"dst,omitempty"`
	Result any     `json:"result,omitempty"`
	Error  *rpcErr `json:"error,omitempty"`
}

// registerGen2HTTP wires /rpc/<Method>, matching the two "parameters only"
// HTTP RPC styles Shelly devices support:
//   - POST /rpc/<Method>  body is just {params}, response is just {result}
//   - GET  /rpc/<Method>?k=v...  scalar params via query string
//
// The "entire frame as payload" POST /rpc (and the WebSocket upgrade on the
// same path) is handled separately in ws_rpc.go.
func registerGen2HTTP(mux *http.ServeMux, st *State) {
	mux.HandleFunc("/rpc/", func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/rpc/")

		var params json.RawMessage
		if r.Method == http.MethodGet {
			params = queryToParams(r)
		} else {
			body, _ := io.ReadAll(r.Body)
			params = body
		}

		result, rerr := dispatchRPC(st, method, params)
		if rerr != nil {
			writeJSON(w, rerr)
			return
		}
		writeJSON(w, result)
	})
}

// queryToParams converts simple scalar query string params (?id=0&on=true)
// into a JSON object, coercing "true"/"false" and numeric-looking values.
// ponytail: scalars only, no dotted-path object assignment
// (config.device.name=...) - add if a consumer actually needs nested config over GET.
func queryToParams(r *http.Request) json.RawMessage {
	params := map[string]any{}
	for k, vals := range r.URL.Query() {
		if len(vals) == 0 {
			continue
		}
		v := vals[0]
		switch {
		case v == "true":
			params[k] = true
		case v == "false":
			params[k] = false
		default:
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				params[k] = n
			} else {
				params[k] = v
			}
		}
	}
	data, _ := json.Marshal(params)
	return data
}
