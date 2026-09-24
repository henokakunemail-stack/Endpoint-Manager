package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	agentupdate "github.com/henokakunemail-stack/Endpoint-Manager/agent/shared/update"
	"github.com/henokakunemail-stack/Endpoint-Manager/server/core/db"
	serverupdate "github.com/henokakunemail-stack/Endpoint-Manager/server/modules/agentupdate"
)

func TestAgentUpdate_ReleaseLifecycle(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := serverupdate.NewRepository(database)
	ctx := context.Background()

	release := &serverupdate.AgentRelease{
		Version:        "1.1.0",
		OSName:         "windows",
		Arch:           "amd64",
		FilePath:       "C:\\dummy\\emagent.exe",
		FileSize:       15485760,
		SHA256Checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Changelog:      "Added remote control and web filter modules",
		IsActive:       true,
		UploadedBy:     "admin-01",
	}

	if err := repo.CreateRelease(ctx, release); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if release.ID == "" {
		t.Fatal("expected generated release ID")
	}

	fetched, err := repo.GetRelease(ctx, release.ID)
	if err != nil {
		t.Fatalf("get release: %v", err)
	}
	if fetched.Version != "1.1.0" || fetched.OSName != "windows" || fetched.Arch != "amd64" {
		t.Fatalf("release metadata mismatch: %+v", fetched)
	}

	activeRel, err := repo.GetActiveReleaseForDevice(ctx, "1.1.0", "windows", "amd64")
	if err != nil {
		t.Fatalf("get active release: %v", err)
	}
	if activeRel.ID != release.ID {
		t.Fatalf("expected ID %s, got %s", release.ID, activeRel.ID)
	}

	releases, err := repo.ListReleases(ctx)
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
}

func TestAgentUpdate_CampaignAndTasks(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := serverupdate.NewRepository(database)
	ctx := context.Background()
	now := time.Now().UTC()

	// Seed test devices and group
	_, err = database.Exec(`
		INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
		VALUES
			('dev-up-1', 'HOST-ALPHA', 'windows', '11.0', '1.0.0', 'Jakarta', 'online', ?, ?, 'h1', ?, ?),
			('dev-up-2', 'HOST-BETA', 'linux', '5.15', '1.0.0', 'Surabaya', 'online', ?, ?, 'h2', ?, ?)
	`, now, now, now, now, now, now, now, now)
	if err != nil {
		t.Fatal(err)
	}

	_, err = database.Exec(`
		INSERT INTO device_groups (id, name, description, created_at, updated_at)
		VALUES ('grp-pilot', 'Pilot Fleet', 'First rollout test devices', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatal(err)
	}

	_, err = database.Exec(`
		INSERT INTO device_group_members (group_id, device_id, added_at)
		VALUES ('grp-pilot', 'dev-up-1', ?)
	`, now)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Create and verify Campaign
	campaign := &serverupdate.UpdateCampaign{
		Name:               "Q3 Agent Upgrade to 1.2.0",
		Description:        "Fleet upgrade for enhanced security monitoring",
		TargetVersion:      "1.2.0",
		TargetType:         "group",
		TargetID:           "grp-pilot",
		BatchSize:          10,
		StaggerIntervalSec: 60,
		CreatedBy:          "admin",
	}

	if err := repo.CreateCampaign(ctx, campaign); err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	targets, err := repo.ResolveTargetDevices(ctx, campaign.TargetType, campaign.TargetID)
	if err != nil {
		t.Fatalf("resolve target devices: %v", err)
	}
	if len(targets) != 1 || targets[0] != "dev-up-1" {
		t.Fatalf("expected target [dev-up-1], got: %v", targets)
	}

	// 2. Create and track task lifecycle
	task := &serverupdate.DeviceUpdateTask{
		CampaignID:    &campaign.ID,
		DeviceID:      "dev-up-1",
		FromVersion:   "1.0.0",
		TargetVersion: "1.2.0",
		Status:        "downloading",
	}
	if err := repo.CreateUpdateTask(ctx, task); err != nil {
		t.Fatalf("create update task: %v", err)
	}

	if err := repo.RecordTaskProgress(ctx, task.ID, "swapping", ""); err != nil {
		t.Fatalf("record swapping: %v", err)
	}

	fetchedTask, err := repo.GetUpdateTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("get update task: %v", err)
	}
	if fetchedTask.Status != "swapping" {
		t.Fatalf("expected swapping, got: %s", fetchedTask.Status)
	}

	// Success and version promotion
	if err := repo.RecordTaskProgress(ctx, task.ID, "success", ""); err != nil {
		t.Fatalf("record success: %v", err)
	}
	if err := repo.UpdateDeviceAgentVersion(ctx, "dev-up-1", "1.2.0"); err != nil {
		t.Fatalf("update device agent version: %v", err)
	}

	var updatedDevVersion string
	err = database.GetContext(ctx, &updatedDevVersion, `SELECT agent_version FROM devices WHERE id = 'dev-up-1'`)
	if err != nil {
		t.Fatal(err)
	}
	if updatedDevVersion != "1.2.0" {
		t.Fatalf("expected device version 1.2.0, got: %s", updatedDevVersion)
	}
}

func TestAgentUpdate_EngineChecksumAndSwap(t *testing.T) {
	tempDir := t.TempDir()

	// Initial mock binary
	mockOriginalBin := filepath.Join(tempDir, "mock_agent.exe")
	if err := os.WriteFile(mockOriginalBin, []byte("ORIGINAL_AGENT_BINARY_V1"), 0755); err != nil {
		t.Fatal(err)
	}

	newBinaryContent := []byte("UPGRADED_AGENT_BINARY_V2_WITH_SECURITY_PATCH")
	hasher := sha256.New()
	hasher.Write(newBinaryContent)
	validChecksum := hex.EncodeToString(hasher.Sum(nil))

	reportedStatuses := make([]string, 0)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download/agent-v2" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(newBinaryContent)
			return
		}
		if r.URL.Path == "/api/agent/devices/test-dev/update/report" {
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			reportedStatuses = append(reportedStatuses, string(buf[:n]))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	engine := agentupdate.NewEngine(mockServer.URL, "test-dev", "secret")
	engine.SetExecutablePath(mockOriginalBin)

	// 1. Success case: valid checksum
	params := agentupdate.UpdateParams{
		TaskID:         "task-123",
		TargetVersion:  "1.2.0",
		DownloadURL:    "/download/agent-v2",
		SHA256Checksum: validChecksum,
		FileSize:       int64(len(newBinaryContent)),
	}

	err := engine.ApplyUpdate(context.Background(), params)
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}

	currentContent, err := os.ReadFile(mockOriginalBin)
	if err != nil {
		t.Fatalf("read current bin: %v", err)
	}
	if string(currentContent) != string(newBinaryContent) {
		t.Fatalf("expected binary content to be upgraded, got: %s", string(currentContent))
	}

	// Verify backup file exists
	backupContent, err := os.ReadFile(mockOriginalBin + ".old")
	if err != nil {
		t.Fatalf("read backup bin: %v", err)
	}
	if string(backupContent) != "ORIGINAL_AGENT_BINARY_V1" {
		t.Fatalf("expected backup to have original content, got: %s", string(backupContent))
	}

	// 2. Failure case: hash mismatch should abort and leave file intact
	badChecksumParams := agentupdate.UpdateParams{
		TaskID:         "task-bad-hash",
		TargetVersion:  "1.3.0",
		DownloadURL:    "/download/agent-v2",
		SHA256Checksum: "0000000000000000000000000000000000000000000000000000000000000000",
		FileSize:       int64(len(newBinaryContent)),
	}

	err = engine.ApplyUpdate(context.Background(), badChecksumParams)
	if err == nil {
		t.Fatal("expected error on hash mismatch, got nil")
	}

	// Content should still be V2 (not corrupted)
	contentAfterBad, _ := os.ReadFile(mockOriginalBin)
	if string(contentAfterBad) != string(newBinaryContent) {
		t.Fatalf("binary modified despite hash mismatch: %s", string(contentAfterBad))
	}
}
