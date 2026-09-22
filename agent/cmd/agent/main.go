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
	"github.com/endpoint-mgmt/agent/shared/inventory"
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

	collector := newInventoryCollector()

	// Commands the agent can run. Later modules register more types here.
	dispatcher := transport.NewDispatcher()
	dispatcher.Register("ping", func(ctx context.Context, command, id string, payload json.RawMessage) any {
		return map[string]string{"pong": time.Now().Format(time.RFC3339)}
	})

	client := transport.NewClient(creds.ServerURL, creds.DeviceID, creds.DeviceSecret)
	client.SetCommandHandler(dispatcher.Handle)

	// On-demand collection runs on the client's goroutine; it reports over the
	// same socket the periodic scheduler uses.
	client.SetCollectHandler(func(ctx context.Context) {
		if err := inventory.CollectOnce(ctx, collector, client); err != nil {
			log.Warn().Err(err).Msg("on-demand inventory collect")
		} else {
			log.Info().Msg("inventory collected on server request")
		}
	})

	// Periodic inventory: collect immediately on startup, then on a stable per-
	// device cadence. The scheduler swallows its own errors so inventory never
	// affects command handling.
	go func() {
		sched := inventory.NewScheduler(collector, client, creds.DeviceID,
			inventoryPeriod(), inventoryStaggerWindow())
		sched.Run(ctx)
	}()

	// Advertise what this build can do, so the server avoids sending commands to
	// an agent that would silently drop them.
	client.SetHelloExtra(map[string]any{
		"capabilities": inventoryCapabilities(),
	})

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

// inventoryPeriod is how long between full collections for one device.
// 4 hours is frequent enough to catch a re-provisioned machine during a work
// day, and rare enough that 500 agents collection at once is not a load event.
func inventoryPeriod() time.Duration { return 4 * time.Hour }

// inventoryStaggerWindow is how widely collection start times are spread.
// The scheduler derives a stable per-device offset from this window, so the
// fleet does not align on the hour even on a fresh rollout.
func inventoryStaggerWindow() time.Duration { return 30 * time.Minute }
