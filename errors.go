package nestor

import (
	"errors"
	"fmt"
)

var ErrNotSupported = fmt.Errorf("nestor: not supported")
var ErrNotFound = fmt.Errorf("nestor: not found")
var ErrNotAllowed = fmt.Errorf("nestor: not allowed")
var ErrNotAvailable = fmt.Errorf("nestor: not available")
var ErrLeaseExpired = fmt.Errorf("nestor: lease expired")
var ErrInvalid = fmt.Errorf("nestor: invalid")

var errUnknownSandbox = errors.New("unknown sandbox implementation, use default one, don't implement Sandbox")
