package main

import (
	"encoding/json"
	"fmt"
)

// rpcErr matches the JSON-RPC-ish error shape Shelly Gen2 devices return.
type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// dispatchRPC is the single entry point for Gen2 methods, shared by the
// HTTP /rpc/*, the WebSocket /rpc endpoint, and MQTT RPC topics.
// Field shapes follow https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly
// and .../Switch.
func dispatchRPC(st *State, method string, params json.RawMessage) (any, *rpcErr) {
	switch method {
	case "Shelly.GetDeviceInfo":
		info := map[string]any{
			"id":      st.deviceID,
			"mac":     "AABBCCDDEEFF",
			"model":   "SNPL-00112EU", // Shelly Plus Plug S
			"gen":     2,
			"fw_id":   "sim-1.0.0",
			"app":     "PlusPlugS",
			"ver":     "1.0.0",
			"auth_en": st.authEnabled,
		}
		if st.authEnabled {
			info["auth_domain"] = st.deviceID
		} else {
			info["auth_domain"] = nil
		}
		return info, nil

	case "Shelly.GetStatus":
		on, powerW, energyWh, uptime := st.Snapshot()
		return map[string]any{
			"switch:0": switchStatus(on, powerW, energyWh),
			"sys": map[string]any{
				"uptime": int(uptime.Seconds()),
			},
		}, nil

	case "Switch.GetStatus":
		var p struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(params, &p)
		on, powerW, energyWh, _ := st.Snapshot()
		return switchStatus(on, powerW, energyWh), nil

	case "Switch.Set":
		var p struct {
			ID int  `json:"id"`
			On bool `json:"on"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcErr{Code: -103, Message: "invalid params"}
		}
		wasOn, _, _, _ := st.Snapshot()
		st.SetRelay(p.On)
		return map[string]any{"was_on": wasOn}, nil

	case "Switch.Toggle":
		wasOn, _, _, _ := st.Snapshot()
		st.Toggle()
		return map[string]any{"was_on": wasOn}, nil

	default:
		return nil, &rpcErr{Code: -105, Message: fmt.Sprintf("unknown method %q", method)}
	}
}

func switchStatus(on bool, powerW, energyWh float64) map[string]any {
	return map[string]any{
		"id":      0,
		"output":  on,
		"apower":  powerW,
		"aenergy": map[string]any{"total": energyWh},
	}
}
