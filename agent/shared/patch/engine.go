package patch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/henokakunemail-stack/Endpoint-Manager/agent/shared/transport"
)

type Engine struct {
	serverBase string
	deviceID   string
	secret     string
	client     *http.Client
}

func NewEngine(serverBase, deviceID, secret string) *Engine {
	base := strings.TrimRight(serverBase, "/")
	return &Engine{
		serverBase: base,
		deviceID:   deviceID,
		secret:     secret,
		client:     transport.NewHTTPClient(30 * time.Second),
	}
}

func (e *Engine) Scan(ctx context.Context) ([]PatchItem, error) {
	return scanOS(ctx)
}

func (e *Engine) Install(ctx context.Context, params InstallParams) (InstallResult, error) {
	return installOS(ctx, params)
}

func (e *Engine) ReportScan(ctx context.Context, patches []PatchItem) error {
	url := fmt.Sprintf("%s/api/agent/patches/scan-report", e.serverBase)
	payload := map[string]any{
		"patches": patches,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal scan report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create report request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", e.deviceID)
	req.Header.Set("X-Device-Secret", e.secret)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("send scan report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("report scan failed with status %d", resp.StatusCode)
	}
	return nil
}

func (e *Engine) ReportInstall(ctx context.Context, res InstallResult) error {
	url := fmt.Sprintf("%s/api/agent/patches/install-result", e.serverBase)
	body, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal install result: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create report request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", e.deviceID)
	req.Header.Set("X-Device-Secret", e.secret)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("send install report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("report install failed with status %d", resp.StatusCode)
	}
	return nil
}
