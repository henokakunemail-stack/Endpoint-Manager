package devicemanagement

import (
	"context"
	"time"
)

// backgroundCtx is used for inventory writes that outlive a single request, such
// as an agent's report arriving over the WebSocket read loop.
func backgroundCtx() context.Context { return context.Background() }

func nowUTC() time.Time { return time.Now().UTC() }

// timeOrZero accepts a timestamp that may be absent, missing, or malformed; an
// unparseable value falls back to the current server time so a report is never
// rejected over a clock-skewed agent.
type timeOrZero time.Time

func (t *timeOrZero) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" || s == `""` {
		*t = timeOrZero(time.Now().UTC())
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		*t = timeOrZero(time.Now().UTC())
		return nil
	}
	*t = timeOrZero(parsed.UTC())
	return nil
}
