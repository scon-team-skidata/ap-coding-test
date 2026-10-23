package main

import (
	"encoding/json"
	"net/http"
)

// registerGen1 wires the classic Shelly REST API: /shelly, /settings,
// /status, /relay/0 (+?turn=on|off|toggle).
func registerGen1(mux *http.ServeMux, st *State) {
	mux.HandleFunc("/shelly", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"type":        "SHPLG-S",
			"mac":         "AABBCCDDEEFF",
			"id":          st.deviceID,
			"auth":        st.authEnabled,
			"fw":          "20230913-sim",
			"num_outputs": 1,
		})
	})

	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"device": map[string]any{"type": "SHPLG-S", "mac": "AABBCCDDEEFF", "hostname": st.deviceID},
			"name":   st.deviceID,
		})
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		on, powerW, energyWh, uptime := st.Snapshot()
		writeJSON(w, map[string]any{
			"relays": []map[string]any{{"ison": on}},
			"meters": []map[string]any{{"power": powerW, "total": energyWh}},
			"uptime": int(uptime.Seconds()),
		})
	})

	mux.HandleFunc("/relay/0", func(w http.ResponseWriter, r *http.Request) {
		if turn := r.URL.Query().Get("turn"); turn != "" {
			switch turn {
			case "on":
				st.SetRelay(true)
			case "off":
				st.SetRelay(false)
			case "toggle":
				st.Toggle()
			default:
				http.Error(w, "invalid turn value", http.StatusBadRequest)
				return
			}
		}
		on, powerW, energyWh, _ := st.Snapshot()
		writeJSON(w, map[string]any{
			"ison":         on,
			"has_timer":    false,
			"power":        powerW,
			"overpower":    false,
			"source":       "http",
			"energy_total": energyWh,
		})
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
