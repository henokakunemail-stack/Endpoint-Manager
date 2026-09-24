package integration

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/endpoint-mgmt/server/core/db"
	"github.com/endpoint-mgmt/server/modules/alerting"
)

func TestAlerting_RulesAndIncidents(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := alerting.NewRepository(database)
	eval := alerting.NewEvaluator(database, repo)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Test Rule Creation & Listing
	rule1 := &alerting.AlertRule{
		Name:         "Low Disk Capacity",
		RuleType:     "disk_low",
		ThresholdVal: 15.0, // < 15%
		Severity:     "critical",
		WebhookURL:   "http://localhost:9999/webhook-test",
		IsEnabled:    true,
		CreatedBy:    "admin-user-id",
	}
	if err := repo.CreateRule(ctx, rule1); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	rules, err := repo.ListRules(ctx)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "Low Disk Capacity" {
		t.Fatalf("expected 1 rule with name 'Low Disk Capacity', got: %+v", rules)
	}
	t.Logf("Rule created and listed: %s (ID: %s)", rules[0].Name, rules[0].ID)

	// 2. Seed Mock Devices and Inventory
	// Device 1: Low disk (8.5% free -> should trigger)
	_, err = database.Exec(`
		INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-low-disk', 'SRV-STORAGE-01', 'windows', '10.0', '1.0.0', 'Jakarta DC', 'online', ?, ?, 'h1', ?, ?)
	`, now, now, now, now)
	if err != nil {
		t.Fatal("seed device 1:", err)
	}

	_, err = database.Exec(`
		INSERT INTO device_inventory (id, device_id, hw, software, os_detail, hw_ram_bytes, hw_disk_free_pct, hw_cpu_model, collected_at, updated_at)
		VALUES ('inv-1', 'dev-low-disk', '{}', '[]', '{}', 34359738368, 8.5, 'Xeon E5-2680', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatal("seed inventory 1:", err)
	}

	// Device 2: Healthy disk (45.0% free -> should NOT trigger)
	_, err = database.Exec(`
		INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-healthy-disk', 'PC-FINANCE-02', 'windows', '11.0', '1.0.0', 'Jakarta Head', 'online', ?, ?, 'h2', ?, ?)
	`, now, now, now, now)
	if err != nil {
		t.Fatal("seed device 2:", err)
	}

	_, err = database.Exec(`
		INSERT INTO device_inventory (id, device_id, hw, software, os_detail, hw_ram_bytes, hw_disk_free_pct, hw_cpu_model, collected_at, updated_at)
		VALUES ('inv-2', 'dev-healthy-disk', '{}', '[]', '{}', 17179869184, 45.0, 'Core i5-1135G7', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatal("seed inventory 2:", err)
	}

	// 3. Trigger Evaluation Cycle 1
	evalRes1, err := eval.EvaluateAll(ctx)
	if err != nil {
		t.Fatalf("evaluate cycle 1: %v", err)
	}
	if evalRes1.IncidentsNew != 1 {
		t.Fatalf("expected 1 new incident, got %d", evalRes1.IncidentsNew)
	}
	t.Logf("Cycle 1 complete: 1 new incident created")

	// 4. Verify Active Incidents
	incidents, err := repo.ListIncidents(ctx, "open", "", 10)
	if err != nil {
		t.Fatalf("list incidents: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 open incident, got %d", len(incidents))
	}
	inc := incidents[0]
	if inc.DeviceID != "dev-low-disk" || inc.Hostname != "SRV-STORAGE-01" || inc.Severity != "critical" {
		t.Fatalf("unexpected incident payload: %+v", inc)
	}
	if inc.TriggerCount != 1 {
		t.Fatalf("expected trigger_count=1, got %d", inc.TriggerCount)
	}
	t.Logf("Incident verified: %s (Device: %s, Triggers: %d)", inc.Title, inc.Hostname, inc.TriggerCount)

	// 5. Test Deduplication (Trigger Evaluation Cycle 2)
	evalRes2, err := eval.EvaluateAll(ctx)
	if err != nil {
		t.Fatalf("evaluate cycle 2: %v", err)
	}
	if evalRes2.IncidentsNew != 0 || evalRes2.IncidentsDedup != 1 {
		t.Fatalf("expected 0 new and 1 dedup, got %+v", evalRes2)
	}

	// Check that trigger_count was incremented to 2
	incAfter, err := repo.GetIncidentByID(ctx, inc.ID)
	if err != nil {
		t.Fatalf("get incident after: %v", err)
	}
	if incAfter.TriggerCount != 2 {
		t.Fatalf("expected trigger_count=2, got %d", incAfter.TriggerCount)
	}
	t.Logf("Deduplication verified: trigger_count incremented to %d without creating duplicate rows", incAfter.TriggerCount)

	// 6. Test Incident Lifecycle: Acknowledge
	if err := repo.AcknowledgeIncident(ctx, inc.ID, "tech-user-id"); err != nil {
		t.Fatalf("acknowledge incident: %v", err)
	}
	incAck, _ := repo.GetIncidentByID(ctx, inc.ID)
	if incAck.Status != "acknowledged" || *incAck.AcknowledgedBy != "tech-user-id" {
		t.Fatalf("expected acknowledged status, got: %+v", incAck)
	}
	t.Logf("Incident acknowledged successfully by %s", *incAck.AcknowledgedBy)

	// 7. Test Incident Lifecycle: Resolve
	if err := repo.ResolveIncident(ctx, inc.ID, "tech-user-id"); err != nil {
		t.Fatalf("resolve incident: %v", err)
	}
	incRes, _ := repo.GetIncidentByID(ctx, inc.ID)
	if incRes.Status != "resolved" || *incRes.ResolvedBy != "tech-user-id" {
		t.Fatalf("expected resolved status, got: %+v", incRes)
	}
	t.Logf("Incident resolved successfully by %s", *incRes.ResolvedBy)

	// Verify open list is now empty
	openIncidents, _ := repo.ListIncidents(ctx, "open", "", 10)
	if len(openIncidents) != 0 {
		t.Fatalf("expected 0 open incidents after resolution, got %d", len(openIncidents))
	}
	t.Logf("All lifecycle states verified: open -> acknowledged -> resolved")
}
