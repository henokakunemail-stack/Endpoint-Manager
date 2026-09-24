package remoteexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type ExecPayload struct {
	ExecutionID string `json:"execution_id"`
	Shell       string `json:"shell"`
	Command     string `json:"command"`
	TimeoutSec  int    `json:"timeout_sec"`
}

type ExecReport struct {
	ExecutionID  string  `json:"execution_id"`
	Status       string  `json:"status"` // completed, failed, timeout
	ExitCode     *int    `json:"exit_code,omitempty"`
	Output       *string `json:"output,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
}

type CommandRunner interface {
	RunCommand(ctx context.Context, shell, command string) (exitCode int, output string, err error)
}

// ToHTTPURL normalizes ws:// and wss:// to http:// and https://.
func ToHTTPURL(serverURL string) string {
	if strings.HasPrefix(serverURL, "wss://") {
		return "https://" + strings.TrimPrefix(serverURL, "wss://")
	}
	if strings.HasPrefix(serverURL, "ws://") {
		return "http://" + strings.TrimPrefix(serverURL, "ws://")
	}
	return strings.TrimRight(serverURL, "/")
}

// ReportResult posts the command execution outcome back to the server.
func ReportResult(ctx context.Context, serverURL, deviceID, deviceSecret string, rep ExecReport) error {
	apiBase := ToHTTPURL(serverURL)
	b, err := json.Marshal(rep)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/api/agent/executions/%s/result", apiBase, rep.ExecutionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", deviceID)
	req.Header.Set("X-Device-Secret", deviceSecret)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("server rejected execution report: status %d", resp.StatusCode)
	}
	return nil
}

// ExecuteAndReport runs a remote command in the background and reports results.
func ExecuteAndReport(ctx context.Context, serverURL, deviceID, deviceSecret string, rawPayload json.RawMessage) error {
	var payload ExecPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	if payload.ExecutionID == "" || payload.Command == "" {
		return errors.New("missing execution_id or command")
	}

	timeout := 60 * time.Second
	if payload.TimeoutSec > 0 {
		timeout = time.Duration(payload.TimeoutSec) * time.Second
	}

	log.Info().
		Str("exec_id", payload.ExecutionID).
		Str("shell", payload.Shell).
		Str("command", payload.Command).
		Dur("timeout", timeout).
		Msg("starting remote execution")

	execCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	runner := newPlatformRunner()
	exitCode, output, err := runner.RunCommand(execCtx, payload.Shell, payload.Command)

	rep := ExecReport{
		ExecutionID: payload.ExecutionID,
		ExitCode:    &exitCode,
		Output:      &output,
	}

	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			rep.Status = "timeout"
			errMsg := fmt.Sprintf("execution timed out after %v", timeout)
			rep.ErrorMessage = &errMsg
		} else {
			rep.Status = "failed"
			errMsg := err.Error()
			rep.ErrorMessage = &errMsg
		}
	} else if exitCode == 0 {
		rep.Status = "completed"
	} else {
		rep.Status = "failed"
		errMsg := fmt.Sprintf("command exited with non-zero code %d", exitCode)
		rep.ErrorMessage = &errMsg
	}

	log.Info().
		Str("exec_id", payload.ExecutionID).
		Str("status", rep.Status).
		Int("exit_code", exitCode).
		Msg("remote execution finished, reporting result")

	reportCtx, reportCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reportCancel()

	return ReportResult(reportCtx, serverURL, deviceID, deviceSecret, rep)
}
