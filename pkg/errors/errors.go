package errors

import (
	"errors"
)

var (
	ErrNoStructField   = errors.New("no struct field")
	ErrUnsupportedKind = errors.New("unsupported kind")
)
