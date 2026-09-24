//go:build windows

package service

import (
	"runtime/debug"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows/svc"
)

// RunAsService reports whether the current process was launched by the Windows
// Service Control Manager rather than from an interactive shell.
//
// Registration alone is not enough: sc.exe can create the service entry, but
// Windows will not transition it to RUNNING unless the process calls svc.Run.
// Without this the service starts, immediately exits, and the SCM reports
// error 1053 (the service did not respond in a timely fashion).
func RunAsService() bool {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isSvc
}

// Serve runs the given agent entrypoint under the Service Control Manager and
// blocks until the service is stopped. It returns an error only when the
// process could not connect to the SCM at all.
func Serve(name string, run func() error) error {
	return svc.Run(name, &scmHandler{run: run})
}

// shutdownGrace bounds how long the handler waits for the agent to unwind after
// a stop request. A hung agent must not keep the SCM in StopPending forever.
const shutdownGrace = 20 * time.Second

// scmHandler adapts a blocking agent entrypoint to the svc.Handler contract.
type scmHandler struct {
	run func() error
	// failed records a non-zero exit so the SCM marks the service as failed
	// rather than reporting a clean stop.
	failed atomic.Bool
}

func (h *scmHandler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if rec := recover(); rec != nil {
				h.failed.Store(true)
				debug.PrintStack()
			}
		}()
		if err := h.run(); err != nil {
			h.failed.Store(true)
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	stop := func() (bool, uint32) {
		changes <- svc.Status{State: svc.StopPending}
		select {
		case <-done:
		case <-time.After(shutdownGrace):
			// The agent did not unwind in time. Report failure so the restart
			// policy can replace a wedged process.
			h.failed.Store(true)
		}
		changes <- svc.Status{State: svc.Stopped}
		if h.failed.Load() {
			return true, 1
		}
		return false, 0
	}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				return stop()
			}
		case <-done:
			// The agent exited on its own; nothing left to supervise.
			changes <- svc.Status{State: svc.Stopped}
			if h.failed.Load() {
				return true, 1
			}
			return false, 0
		}
	}
}
