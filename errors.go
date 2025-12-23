package envParser

import (
	"errors"
)

var (
	// ErrInvalidEnviron returned when the environ is not valid.
	ErrInvalidEnviron = errors.New("items environ must have valid format key=value")

	// ErrInvalidValue returned when the value is not a pointer to a struct.
	ErrInvalidValue = errors.New("value must be a non-nil pointer to a struct")

	// ErrUnsupportedType returned when a field with tag is unsupported.
	ErrUnsupportedType = errors.New("field is an unsupported type")
)
