package errors

import "errors"

var (
	ErrUnsupportedIndexType = errors.New("unsupported index type")
	ErrNilContext = errors.New("nil context")
)
