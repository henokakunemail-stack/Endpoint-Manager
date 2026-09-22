package devicemanagement

import "errors"

// ErrNotFound means the device (or token) does not exist.
var ErrNotFound = errors.New("device not found")

// ErrInvalidSecret means the presented device secret did not match.
var ErrInvalidSecret = errors.New("invalid device secret")
