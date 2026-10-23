package main

import "context"

// device and publisher are optional boundaries for testing orchestration with
// small fakes. You may change or remove them; discovery and connection lifecycle
// remain on the concrete clients, and no framework or embedding is required.
type device interface {
	status(context.Context) (telemetry, error)
	setRelay(context.Context, bool) error
}

type publisher interface {
	publishTelemetry(context.Context, telemetry) error
}

// run owns the agent lifecycle. This suggested structure may be changed.
func run(ctx context.Context) error {
	// TODO: Parse and validate configuration.
	// TODO: Discover the simulator device over Gen1 HTTP.
	// TODO: Connect to MQTT and restore command subscriptions on reconnect.
	// TODO: Poll and publish telemetry; serialize validated relay commands.
	// TODO: Bound network operations and clean up resources on cancellation.
	<-ctx.Done()
	return nil
}
