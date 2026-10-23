package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// ponytail: allow any origin, this is a local dev simulator not a public service
	CheckOrigin: func(r *http.Request) bool { return true },
}

// registerWSRPC wires /rpc, which (per Shelly's RPC Channels doc) serves
// both the WebSocket upgrade and the HTTP "entire frame as payload" POST
// on the same path.
func registerWSRPC(mux *http.ServeMux, st *State) {
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			serveWS(w, r, st)
			return
		}
		if r.Method == http.MethodPost {
			serveFullFrameRPC(w, r, st)
			return
		}
		http.NotFound(w, r)
	})
}

func serveFullFrameRPC(w http.ResponseWriter, r *http.Request, st *State) {
	body, _ := io.ReadAll(r.Body)
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, rpcResponse{Error: &rpcErr{Code: -103, Message: "invalid request frame"}})
		return
	}
	result, rerr := dispatchRPC(st, req.Method, req.Params)
	writeJSON(w, rpcResponse{ID: req.ID, Src: st.deviceID, Dst: req.Src, Result: result, Error: rerr})
}

func serveWS(w http.ResponseWriter, r *http.Request, st *State) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	for {
		var req rpcRequest
		if err := conn.ReadJSON(&req); err != nil {
			return // client disconnected or sent garbage
		}
		result, rerr := dispatchRPC(st, req.Method, req.Params)
		resp := rpcResponse{ID: req.ID, Src: st.deviceID, Dst: req.Src, Result: result, Error: rerr}
		if err := conn.WriteJSON(resp); err != nil {
			return
		}
	}
}
