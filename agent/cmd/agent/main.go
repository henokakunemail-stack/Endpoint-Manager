// Command endpoint-mgmt-agent is the multi-OS endpoint agent.
//
// Usage:
//
//	endpoint-mgmt-agent -enroll <one-time-token>   # first run: join the fleet
//	endpoint-mgmt-agent                            # subsequent runs: connect
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/endpoint-mgmt/agent/shared/enrollment"
	"github.com/endpoint-mgmt/agent/shared/transport"
)

// osProvider is supplied per-OS by the build-tagged package for the target platform.
// We import the platform package anonymously so its init/provider is linked in.
var osProvider = newOSInfoProvider()

func main() {
	var (
		serverURL     string
		enrollToken   string
		credsPath     string
		heartbeatSecs int
	)
	flag.StringVar(&serverURL, "server", envOr("AGENT_SERVER", "http://localhost:8443"), "central server URL")
	flag.StringVar(&enrollToken, "enroll", "", "one-time enrollment token (first run only)")
	flag.StringVar(&credsPath, "creds", defaultCredsPath(), "path to persisted credentials")
	flag.IntVar(&heartbeatSecs, "heartbeat", 20, "heartbeat interval in seconds")
	flag.Parse()

	log.Info().Str("server", serverURL).Str("creds", credsPath).Msg("agent starting")

	creds, err := enrollment.Load(credsPath)
	if enrollToken != "" {
		if err != nil && err != enrollment.ErrNotEnrolled {
			log.Fatal().Err(err).Msg("load existing credentials")
		}
		log.Info().Msg("enrolling with provided token")
		creds, err = enrollment.Exchange(serverURL, enrollToken)
		if err != nil {
			log.Fatal().Err(err).Msg("enrollment failed")
		}
		if err := enrollment.Save(credsPath, creds); err != nil {
			log.Fatal().Err(err).Msg("persist credentials")
		}
		log.Info().Str("device_id", creds.DeviceID).Msg("enrolled successfully")
	} else if err != nil {
		log.Fatal().Err(err).Msg("not enrolled; run with -enroll <token>")
	}

	info, err := osProvider.Collect()
	if err != nil {
		log.Fatal().Err(err).Msg("collect os info")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Commands the agent can run. Later modules register more types here.
	dispatcher := transport.NewDispatcher()
	dispatcher.Register("ping", func(ctx context.Context, command, id string, payload json.RawMessage) any {
		return map[string]string{"pong": time.Now().Format(time.RFC3339)}
	})

	client := transport.NewClient(creds.ServerURL, creds.DeviceID, creds.DeviceSecret)
	client.SetCommandHandler(dispatcher.Handle)

	// On shutdown, close the socket so the server marks the device offline
	// promptly instead of waiting for a ping/pong deadline.
	go func() {
		<-ctx.Done()
		log.Info().Msg("shutdown signal received, closing connection")
		client.Close()
	}()

	if err := client.Run(ctx, info); err != nil && ctx.Err() == nil {
		log.Error().Err(err).Msg("transport ended")
	}
	log.Info().Msg("agent stopped")
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func defaultCredsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return fmt.Sprintf("%s/.endpoint-mgmt/agent-creds.json", home)
}
