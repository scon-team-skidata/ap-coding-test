package main

import (
	"context"
	"testing"
	"time"
)

func TestSimulationWithoutMQTT(t *testing.T) {
	st := newState("offline-plug", false)
	st.SetRelay(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		deviceTicker(ctx, nil, st, time.Millisecond)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		_, power, energy, _ := st.Snapshot()
		if power > 0 && energy > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("simulation did not advance without MQTT")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("simulation did not stop on cancellation")
	}
	_, _, accumulated, _ := st.Snapshot()
	st.SetRelay(false)
	st.tick(time.Second)
	_, power, energy, _ := st.Snapshot()
	if power != 0 || energy != accumulated {
		t.Fatal("off relay should draw no power and preserve accumulated energy")
	}
}
