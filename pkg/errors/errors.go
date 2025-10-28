package errors

import (
	"errors"
)

var (
	ErrNilGenericTypeInfo = errors.New("nil generic type info")
	// TODO: Move?
	ErrNilField      = errors.New("nil field")
	ErrNoStructField = errors.New("no struct field")
	ErrUnsupportedKind = errors.New("unsupported kind")
)
