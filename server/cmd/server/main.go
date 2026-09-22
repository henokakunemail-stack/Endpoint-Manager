// Command endpoint-mgmt-server is the central management server (Fase 1).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/endpoint-mgmt/server/core/audit"
	"github.com/endpoint-mgmt/server/core/auth"
	"github.com/endpoint-mgmt/server/core/config"
	"github.com/endpoint-mgmt/server/core/db"
	"github.com/endpoint-mgmt/server/core/logger"
	"github.com/endpoint-mgmt/server/core/rbac"
	"github.com/endpoint-mgmt/server/core/transport"
	devicemgmt "github.com/endpoint-mgmt/server/modules/device-management"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Logger not initialized yet; write to stderr directly.
		println("config error:", err.Error())
		os.Exit(1)
	}
	logger.Init(cfg.LogLevel)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Str("db", cfg.DBPath).Msg("open database")
	}
	defer database.Close()

	// Fase 1 bootstrap: ensure an admin user exists so the console can log in.
	if err := bootstrapAdmin(database); err != nil {
		log.Fatal().Err(err).Msg("bootstrap admin")
	}

	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	deviceRepo := devicemgmt.NewRepository(database)
	hub := transport.NewHub()

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "agents_online": hub.Count()})
	})

	// Admin console auth (public endpoints).
	loginH := auth.NewLoginHandler(database, jwtSvc)
	loginH.Register(r)

	// Agent endpoints (authenticated by per-device secret, not JWT).
	enrollH := devicemgmt.NewEnrollmentHandler(deviceRepo, database)
	enrollH.Register(r)

	r.Handle("/api/agent/connect", transport.NewWSHandler(hub, deviceRepo, database, cfg.AgentOfflineAfter))

	// Device management API (JWT + RBAC).
	deviceH := devicemgmt.NewHandler(deviceRepo, database, jwtSvc, cfg.EnrollmentTTL)
	deviceH.Register(r)

	// Command dispatch demo endpoint: send "ping" to a device's live connection.
	r.With(jwtSvc.RequireAuth, rbac.RequireRole(rbac.RoleTechnician)).
		Post("/api/devices/{id}/ping", func(w http.ResponseWriter, r *http.Request) {
			deviceID := chi.URLParam(r, "id")
			// Confirm the device exists before queueing anything.
			if _, err := deviceRepo.GetByID(r.Context(), deviceID); err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
				return
			}
			// Persist the command first, so the agent's later reply has a row to
			// update even if the connection drops in between.
			cmdID := devicemgmt.NewID()
			now := time.Now().UTC()
			if _, err := database.ExecContext(r.Context(), `
				INSERT INTO agent_commands (id, device_id, command_type, payload, status, created_at, sent_at)
				VALUES (?, ?, 'ping', '{}', 'sent', ?, ?)`,
				cmdID, deviceID, now, now); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			// If the agent is not connected the command stays queued as 'sent'
			// rather than being lost — it can be redelivered on reconnect.
			if !hub.Online(deviceID) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "queued", "command_id": cmdID})
				return
			}
			if !hub.SendTo(deviceID, mustJSON(transport.Envelope{
				Type:    transport.TypeCommand,
				ID:      cmdID,
				Command: "ping",
			})) {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "device connection is busy"})
				return
			}
			_ = audit.Log(r.Context(), database, "user",
				auth.UserIDFromContext(r.Context()), "command.send", deviceID,
				map[string]string{"command_type": "ping", "command_id": cmdID})
			writeJSON(w, http.StatusOK, map[string]string{"status": "sent", "command_id": cmdID})
		})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Background sweeper: mark devices whose agents went silent as offline.
	go runOfflineSweep(database, hub, cfg.AgentOfflineAfter)

	go func() {
		log.Info().Str("addr", cfg.HTTPAddr).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("http server")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info().Msg("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown")
	}
}

// bootstrapAdmin creates the default admin if no users exist yet.
// Credentials are printed once to the server log — change after first login.
func bootstrapAdmin(d *sqlx.DB) error {
	var count int
	if err := d.Get(&count, `SELECT COUNT(*) FROM users`); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := auth.HashPassword("admin12345")
	if err != nil {
		return err
	}
	_, err = d.Exec(`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		devicemgmt.NewID(), "admin", hash, rbac.RoleAdmin, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return err
	}
	log.Info().Str("username", "admin").Str("password", "admin12345").
		Msg("bootstrap: created default admin (CHANGE PASSWORD NOW)")
	return nil
}

// runOfflineSweep periodically flags devices as offline when their agents have
// not been heard from within the threshold. The hub is authoritative for liveness:
// a device with a live socket is never marked offline here even during a
// transient heartbeat stall, and a device whose socket is gone is marked offline
// by the disconnect handler itself. This sweeper catches the remaining case —
// a socket that died silently (e.g. NAT timeout) without a close frame.
func runOfflineSweep(d *sqlx.DB, hub *transport.Hub, threshold time.Duration) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		var ids []string
		err := d.Select(&ids, `SELECT id FROM devices WHERE status = 'online'`)
		if err != nil {
			log.Debug().Err(err).Msg("offline sweep: list online devices")
			continue
		}
		now := time.Now().UTC()
		for _, id := range ids {
			if hub.Online(id) {
				continue // live socket — trust it over last_seen
			}
			if _, err := d.Exec(`UPDATE devices SET status = 'offline', updated_at = ? WHERE id = ?`,
				now, id); err != nil {
				log.Debug().Err(err).Str("device", id).Msg("offline sweep: mark")
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error"}`)
	}
	return b
}
