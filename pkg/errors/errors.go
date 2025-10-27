package errors

import "errors"

var (
	ErrNilGenericTypeInfo = errors.New("nil generic type info")
	ErrNilTypeDeclaration = errors.New("nil type declaration")
	// TODO: Move?
	ErrNilField = errors.New("nil field")
	ErrNoStructField = errors.New("no struct field")
)
