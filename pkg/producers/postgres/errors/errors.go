package errors

import "errors"

var (
	ErrGenericTypesUnsupported = errors.New("generic types unsupported")
	ErrNilType                 = errors.New("nil type")
)
