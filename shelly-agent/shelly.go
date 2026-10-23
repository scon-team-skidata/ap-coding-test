package main

import (
	"context"
	"errors"
	"net/http"
)

// shellyClient is a suggested boundary for Gen1 HTTP simulator access.
// The fields and method signatures may be changed to suit your design.
type shellyClient struct {
	http    *http.Client
	baseURL string
	id      string // set by deviceID after successful discovery
}

func newShellyClient(cfg config) (*shellyClient, error) {
	// TODO: Configure a reusable HTTP client with appropriate timeouts.
	return nil, errors.New("TODO: implement simulator client construction")
}

func (s *shellyClient) deviceID(ctx context.Context) (string, error) {
	// TODO: GET /shelly and validate the returned device ID.
	return "", errors.New("TODO: implement device discovery")
}

func (s *shellyClient) status(ctx context.Context) (telemetry, error) {
	// TODO: GET /status, validate relay/meter arrays and fields, and map to telemetry.
	// The simulator reports power in W and total energy in Wh.
	return telemetry{}, errors.New("TODO: implement status retrieval")
}

func (s *shellyClient) setRelay(ctx context.Context, on bool) error {
	// TODO: GET /relay/0?turn=on|off and handle unsuccessful responses.
	return errors.New("TODO: implement relay control")
}
