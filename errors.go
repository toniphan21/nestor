package nestor

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrNotSupported = fmt.Errorf("nestor: not supported")
	ErrNotFound     = fmt.Errorf("nestor: not found")
	ErrNotAllowed   = fmt.Errorf("nestor: not allowed")
	ErrNotAvailable = fmt.Errorf("nestor: not available")
	ErrLeaseExpired = fmt.Errorf("nestor: lease expired")
	ErrInvalid      = fmt.Errorf("nestor: invalid")
)

var errUnknownSandbox = errors.New("unknown sandbox implementation, use default one, don't implement Sandbox")

type ParseError struct {
	DataType string
	File     string
	Err      error
}

func (e *ParseError) Error() string {
	var msg strings.Builder
	switch {
	case e.File != "":
		fmt.Fprintf(&msg, "invalid yaml %s", e.File)
	case e.DataType != "":
		fmt.Fprintf(&msg, "cannot parse %s config", e.DataType)
	}
	msg.WriteRune('\n')

	if te, ok := errors.AsType[*yaml.TypeError](e.Err); ok {
		for _, v := range te.Errors {
			msg.WriteString("  ")
			msg.WriteString(v)
			msg.WriteRune('\n')
		}
		return msg.String()
	}
	return e.Err.Error()
}

func (e *ParseError) Unwrap() error {
	return e.Err
}
