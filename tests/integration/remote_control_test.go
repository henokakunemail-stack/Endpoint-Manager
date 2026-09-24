package integration

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	agentrc "github.com/henokakunemail-stack/Endpoint-Manager/agent/shared/remotecontrol"
	"github.com/henokakunemail-stack/Endpoint-Manager/server/core/db"
	"github.com/henokakunemail-stack/Endpoint-Manager/server/modules/remotecontrol"
)

func TestRemoteControl_SessionLifecycle(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := remotecontrol.NewRepository(database)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Seed user and device
	_, err = database.Exec(`
		INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES ('tech-user-1', 'john.doe', 'hash', 'technician', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-rc-1', 'DESKTOP-FIN-01', 'windows', '11.0', '1.0.0', 'HQ-Finance', 'online', ?, ?, 'hash', ?, ?)
	`, now, now, now, now)
	if err != nil {
		t.Fatalf("seed device: %v", err)
	}

	// 2. Create Remote Control Session (full_control)
	session := &remotecontrol.RemoteControlSession{
		DeviceID:    "dev-rc-1",
		OperatorID:  "tech-user-1",
		SessionMode: "full_control",
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID == "" {
		t.Fatal("expected session ID to be populated")
	}
	if session.Status != "active" {
		t.Fatalf("expected status active, got %s", session.Status)
	}
	t.Logf("Session created: %s (Mode: %s, Device: %s)", session.ID, session.SessionMode, session.DeviceID)

	// 3. Get Session by ID and verify join with devices and users
	fetched, err := repo.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if fetched.Hostname != "DESKTOP-FIN-01" {
		t.Fatalf("expected hostname DESKTOP-FIN-01, got: %s", fetched.Hostname)
	}
	if fetched.OperatorName != "john.doe" {
		t.Fatalf("expected operator name john.doe, got: %s", fetched.OperatorName)
	}
	if fetched.Site != "HQ-Finance" {
		t.Fatalf("expected site HQ-Finance, got: %s", fetched.Site)
	}

	// 4. End Session with telemetry statistics
	frames := 150
	bytes := int64(1048576) // 1 MB
	inputs := 42
	if err := repo.EndSession(ctx, session.ID, frames, bytes, inputs); err != nil {
		t.Fatalf("end session: %v", err)
	}

	ended, err := repo.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("get ended session: %v", err)
	}
	if ended.Status != "ended" {
		t.Fatalf("expected status ended, got: %s", ended.Status)
	}
	if ended.FramesTransacted != frames {
		t.Fatalf("expected %d frames, got %d", frames, ended.FramesTransacted)
	}
	if ended.BytesTransacted != bytes {
		t.Fatalf("expected %d bytes, got %d", bytes, ended.BytesTransacted)
	}
	if ended.InputEventsCount != inputs {
		t.Fatalf("expected %d inputs, got %d", inputs, ended.InputEventsCount)
	}
	if ended.EndedAt == nil {
		t.Fatal("expected ended_at timestamp to be set")
	}

	// 5. List Sessions by Device
	list, err := repo.ListSessionsByDevice(ctx, "dev-rc-1", 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].ID != session.ID {
		t.Fatalf("expected session ID %s, got %s", session.ID, list[0].ID)
	}
}

func TestRemoteControl_RelayManager(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := remotecontrol.NewRepository(database)
	relayMgr := remotecontrol.NewRelayManager(repo)
	ctx := context.Background()
	now := time.Now().UTC()

	// Seed user and device
	_, _ = database.Exec(`
		INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES ('tech-user-2', 'sarah.connor', 'hash', 'technician', ?, ?)
	`, now, now)
	_, _ = database.Exec(`
		INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-rc-2', 'SERVER-OPS-01', 'linux', '5.15', '1.0.0', 'HQ-DataCenter', 'online', ?, ?, 'hash', ?, ?)
	`, now, now, now, now)

	session := &remotecontrol.RemoteControlSession{
		DeviceID:    "dev-rc-2",
		OperatorID:  "tech-user-2",
		SessionMode: "view_only",
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	relay := relayMgr.RegisterSession(session.ID, session.SessionMode)
	if relay == nil {
		t.Fatal("expected relay to be created")
	}

	// Test Frame Forwarding (Agent -> Operator)
	fakeFrame := []byte{0x00, 0x10, 0x00, 0x20, 0xFF, 0xD8, 0xFF, 0xE0} // mock jpeg
	_ = relay.ForwardAgentFrame(2, fakeFrame)

	// In view_only mode, operator input should be discarded and not counted
	fakeInput := []byte(`{"type":"mouse","action":"click","x":100,"y":200}`)
	_ = relay.ForwardOperatorInput(1, fakeInput)

	// Close relay and verify DB persistence
	relayMgr.CloseRelay(session.ID)

	ended, err := repo.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("get ended session: %v", err)
	}
	if ended.Status != "ended" {
		t.Fatalf("expected status ended, got %s", ended.Status)
	}
	if ended.FramesTransacted != 1 {
		t.Fatalf("expected 1 frame recorded, got %d", ended.FramesTransacted)
	}
	if ended.InputEventsCount != 0 {
		t.Fatalf("expected 0 input events in view_only mode, got %d", ended.InputEventsCount)
	}
}

func TestRemoteControl_PlatformCapturer(t *testing.T) {
	capturer := agentrc.NewPlatformCapturer()
	if capturer == nil {
		t.Fatal("expected platform capturer to be created")
	}

	frame, width, height, err := capturer.CaptureScreen()
	if err != nil {
		t.Logf("Platform screen capture returned: %v (expected if running in headless CI/non-interactive desktop)", err)
	} else {
		if width <= 0 || height <= 0 {
			t.Fatalf("invalid screen dimensions: %dx%d", width, height)
		}
		if len(frame) == 0 {
			t.Fatal("expected non-empty frame data")
		}
		t.Logf("Screen captured: %dx%d, %d bytes JPEG", width, height, len(frame))
	}

	// Test Input injection methods (should not crash)
	_ = capturer.InjectMouseEvent("move", 100, 100, "", 0)
	_ = capturer.InjectKeyboardEvent("up", "A", 65)
}
