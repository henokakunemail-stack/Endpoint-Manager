package integration

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/henokakunemail-stack/Endpoint-Manager/server/core/db"
	"github.com/henokakunemail-stack/Endpoint-Manager/server/modules/assetlicense"
)

func TestAssetManagement_Lifecycle(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := assetlicense.NewRepository(database)
	ctx := context.Background()
	now := time.Now().UTC()

	// Seed test device
	_, err = database.Exec(`
		INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-asset-1', 'NB-FIN-01', 'windows', '11.0', '1.0.0', 'Jakarta', 'online', ?, ?, 'h1', ?, ?)
	`, now, now, now, now)
	if err != nil {
		t.Fatal(err)
	}

	purchaseDate := now.AddDate(-1, 0, 0)
	warrantyDate := now.AddDate(0, 0, 20) // in 20 days -> expiring soon

	devID := "dev-asset-1"
	asset := &assetlicense.HardwareAsset{
		AssetTag:          "AST-JKT-0042",
		DeviceID:          &devID,
		ModelName:         "Dell Latitude 3420",
		SerialNumber:      "DELL-SN-998877",
		Vendor:            "Dell Official Partner",
		Site:              "Jakarta-HQ",
		Department:        "Finance",
		AssignedUser:      "Budi Santoso",
		PurchaseDate:      &purchaseDate,
		PurchaseCost:      14500000.0,
		WarrantyExpiresAt: &warrantyDate,
		Status:            "in_use",
		Notes:             "Primary workstation for accounting lead",
	}

	// 1. Create
	if err := repo.CreateAsset(ctx, asset); err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if asset.ID == "" {
		t.Fatal("expected generated asset ID")
	}

	// 2. Get by ID & Tag
	fetched, err := repo.GetAsset(ctx, asset.ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if fetched.AssetTag != "AST-JKT-0042" || fetched.PurchaseCost != 14500000.0 {
		t.Fatalf("asset mismatch: %+v", fetched)
	}

	fetchedByTag, err := repo.GetAssetByTag(ctx, "AST-JKT-0042")
	if err != nil {
		t.Fatalf("get asset by tag: %v", err)
	}
	if fetchedByTag.ID != asset.ID {
		t.Fatalf("expected ID %s, got %s", asset.ID, fetchedByTag.ID)
	}

	// 3. List
	list, err := repo.ListAssets(ctx, "Jakarta-HQ", "in_use")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(list))
	}

	// 4. Update
	asset.Status = "in_repair"
	asset.Notes = "Screen replacement in progress"
	if err := repo.UpdateAsset(ctx, asset); err != nil {
		t.Fatalf("update asset: %v", err)
	}

	updated, err := repo.GetAsset(ctx, asset.ID)
	if err != nil {
		t.Fatalf("get updated: %v", err)
	}
	if updated.Status != "in_repair" || updated.Notes != "Screen replacement in progress" {
		t.Fatalf("updated asset mismatch: %+v", updated)
	}

	// 5. Asset Summary
	summary, err := repo.GetAssetSummary(ctx)
	if err != nil {
		t.Fatalf("get asset summary: %v", err)
	}
	if summary.TotalAssets != 1 || summary.TotalValuation != 14500000.0 || summary.WarrantyExpiringCount != 1 {
		t.Fatalf("summary mismatch: %+v", summary)
	}

	// 6. Delete
	if err := repo.DeleteAsset(ctx, asset.ID); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	_, err = repo.GetAsset(ctx, asset.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestLicenseManagement_Compliance(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	repo := assetlicense.NewRepository(database)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Seed devices
	for i := 1; i <= 4; i++ {
		_, err = database.Exec(`
			INSERT INTO devices (id, hostname, os_name, os_version, agent_version, site, status, enrolled_at, last_seen_at, device_secret_hash, created_at, updated_at)
			VALUES (?, ?, 'windows', '11.0', '1.0.0', 'Jakarta', 'online', ?, ?, 'h', ?, ?)
		`, "dev-"+string(rune('0'+i)), "PC-"+string(rune('0'+i)), now, now, now, now)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 2. Seed installed software detected by inventory
	// Dev 1, 2, 3 have "Office Productivity Suite"
	// Dev 1, 2 have "Specialized CAD Engine" (2 devices)
	devSoftware := map[string]string{
		"dev-1": `[{"name":"Office Productivity Suite 2024"},{"name":"Specialized CAD Engine v10"}]`,
		"dev-2": `[{"name":"Office Productivity Suite 2024"},{"name":"Specialized CAD Engine v10"}]`,
		"dev-3": `[{"name":"Office Productivity Suite 2024"}]`,
		"dev-4": `[]`,
	}
	for dID, swJSON := range devSoftware {
		_, err = database.Exec(`
			INSERT INTO device_inventory (id, device_id, hw, software, os_detail, collected_at, updated_at)
			VALUES (?, ?, '{}', ?, '{}', ?, ?)
		`, "inv-"+dID, dID, swJSON, now, now)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 3. Create License A (Office Suite: 5 seats purchased -> 3 used -> COMPLIANT)
	licA := &assetlicense.SoftwareLicense{
		SoftwareName: "Office Productivity Suite",
		Publisher:    "Enterprise Vendor",
		LicenseType:  "per_device",
		TotalSeats:   5,
		Cost:         5000000.0,
	}
	if err := repo.CreateLicense(ctx, licA); err != nil {
		t.Fatalf("create license A: %v", err)
	}

	// 4. Create License B (CAD Engine: 1 seat purchased -> 2 installed -> OVER_ALLOCATED VIOLATION!)
	licB := &assetlicense.SoftwareLicense{
		SoftwareName: "Specialized CAD Engine",
		Publisher:    "Engineering Soft",
		LicenseType:  "per_device",
		TotalSeats:   1,
		Cost:         25000000.0,
	}
	if err := repo.CreateLicense(ctx, licB); err != nil {
		t.Fatalf("create license B: %v", err)
	}

	// 5. Create License C (Expiring soon in 15 days)
	expSoon := now.AddDate(0, 0, 15)
	licC := &assetlicense.SoftwareLicense{
		SoftwareName: "Antivirus Corporate",
		Publisher:    "SecCorp",
		LicenseType:  "subscription",
		TotalSeats:   10,
		Cost:         3000000.0,
		ExpiresAt:    &expSoon,
	}
	if err := repo.CreateLicense(ctx, licC); err != nil {
		t.Fatalf("create license C: %v", err)
	}

	// 6. Test Explicit Allocation
	if err := repo.AllocateLicense(ctx, licA.ID, "dev-1", "admin"); err != nil {
		t.Fatalf("allocate license: %v", err)
	}
	allocs, err := repo.GetAllocationsForLicense(ctx, licA.ID)
	if err != nil {
		t.Fatalf("get allocations: %v", err)
	}
	if len(allocs) != 1 || allocs[0] != "dev-1" {
		t.Fatalf("allocation mismatch: %v", allocs)
	}

	// 7. Compute Compliance
	compliance, err := repo.ComputeCompliance(ctx)
	if err != nil {
		t.Fatalf("compute compliance: %v", err)
	}
	if len(compliance) != 3 {
		t.Fatalf("expected 3 compliance summaries, got %d", len(compliance))
	}

	statusMap := make(map[string]string)
	for _, c := range compliance {
		statusMap[c.SoftwareName] = c.Status
		t.Logf("Compliance result: %s - Seats: %d, Installed: %d, Status: %s",
			c.SoftwareName, c.TotalSeats, c.InstalledDetected, c.Status)
	}

	if statusMap["Office Productivity Suite"] != "compliant" {
		t.Fatalf("expected Office Suite to be compliant, got: %s", statusMap["Office Productivity Suite"])
	}
	if statusMap["Specialized CAD Engine"] != "over_allocated" {
		t.Fatalf("expected CAD Engine to be over_allocated, got: %s", statusMap["Specialized CAD Engine"])
	}
	if statusMap["Antivirus Corporate"] != "expiring_soon" {
		t.Fatalf("expected Antivirus to be expiring_soon, got: %s", statusMap["Antivirus Corporate"])
	}
}
