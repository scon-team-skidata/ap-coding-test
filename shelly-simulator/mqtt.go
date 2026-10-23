package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// startMQTT connects to the broker and wires both Gen1 and Gen2 topics on
// the same shared State. Simulation is started separately from MQTT so that
// HTTP readings continue to evolve when the broker is unavailable.
func startMQTT(cfg Config, st *State) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MQTT.Broker).
		SetClientID(st.deviceID).
		SetUsername(cfg.MQTT.Username).
		SetPassword(cfg.MQTT.Password).
		SetAutoReconnect(true)

	client := mqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		return nil, tok.Error()
	}

	// Gen1: shellies/<id>/relay/0/command -> on|off|toggle
	cmdTopic := fmt.Sprintf("shellies/%s/relay/0/command", st.deviceID)
	if tok := client.Subscribe(cmdTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		switch string(m.Payload()) {
		case "on":
			st.SetRelay(true)
		case "off":
			st.SetRelay(false)
		case "toggle":
			st.Toggle()
		}
		publishGen1State(client, st)
	}); tok.Wait() && tok.Error() != nil {
		log.Printf("[%s] mqtt subscribe %s failed: %v", st.deviceID, cmdTopic, tok.Error())
	}

	// Gen2: <id>/rpc -> JSON-RPC request; response published to <src>/rpc
	// per https://shelly-api-docs.shelly.cloud/gen2/General/RPCChannels#mqtt
	rpcTopic := fmt.Sprintf("%s/rpc", st.deviceID)
	if tok := client.Subscribe(rpcTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		var req rpcRequest
		if err := json.Unmarshal(m.Payload(), &req); err != nil || req.Src == "" {
			return
		}
		result, rerr := dispatchRPC(st, req.Method, req.Params)
		resp := rpcResponse{ID: req.ID, Src: st.deviceID, Dst: req.Src, Result: result, Error: rerr}
		payload, _ := json.Marshal(resp)
		client.Publish(fmt.Sprintf("%s/rpc", req.Src), 0, false, payload)
	}); tok.Wait() && tok.Error() != nil {
		log.Printf("[%s] mqtt subscribe %s failed: %v", st.deviceID, rpcTopic, tok.Error())
	}

	client.Publish(fmt.Sprintf("%s/online", st.deviceID), 0, true, "true")
	publishGen1State(client, st)

	return client, nil
}

func publishGen1State(client mqtt.Client, st *State) {
	on, powerW, energyWh, _ := st.Snapshot()
	state := "off"
	if on {
		state = "on"
	}
	client.Publish(fmt.Sprintf("shellies/%s/relay/0", st.deviceID), 0, true, state)
	client.Publish(fmt.Sprintf("shellies/%s/relay/0/power", st.deviceID), 0, false, fmt.Sprintf("%.2f", powerW))
	client.Publish(fmt.Sprintf("shellies/%s/relay/0/energy", st.deviceID), 0, false, fmt.Sprintf("%.2f", energyWh))
}

func publishGen2Notify(client mqtt.Client, st *State) {
	on, powerW, energyWh, _ := st.Snapshot()
	payload, _ := json.Marshal(map[string]any{
		"src":    st.deviceID,
		"method": "NotifyStatus",
		"params": map[string]any{"switch:0": switchStatus(on, powerW, energyWh)},
	})
	client.Publish(fmt.Sprintf("%s/events/rpc", st.deviceID), 0, false, payload)
}

func deviceTicker(ctx context.Context, client mqtt.Client, st *State, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if changed := st.tick(interval); changed && client != nil && client.IsConnectionOpen() {
				publishGen1State(client, st)
				publishGen2Notify(client, st)
			}
		}
	}
}
