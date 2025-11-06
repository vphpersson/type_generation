package types

import (
	"fmt"

	"github.com/vphpersson/type_generation/pkg/types/type_declaration"
)

type Type interface {
	String() (string, error)
}
type BasicType string

func (b BasicType) String() (string, error) { return string(b), nil }

const (
	Boolean         = BasicType("boolean")
	Text            = BasicType("text")
	Real            = BasicType("real")
	DoublePrecision = BasicType("double precision")
	SmallInt        = BasicType("smallint")
	Integer         = BasicType("integer")
	BigInt          = BasicType("bigint")
	Timestamp       = BasicType("timestamptz")
	ByteA           = BasicType("bytea")
	CiText          = BasicType("citext")
)

type TypeReference struct {
	TypeDeclaration type_declaration.TypeDeclaration
}

func (t *TypeReference) String() (string, error) {
	return fmt.Sprintf("uuid REFERENCES %s(id)", t.TypeDeclaration.QualifiedName()), nil
}

type ArrayType struct {
	ItemsType Type
}

func (a *ArrayType) String() (string, error) {
	typeStr, err := a.ItemsType.String()
	if err != nil {
		return "", fmt.Errorf("items type string: %w", err)
	}

	return fmt.Sprintf("%s[]", typeStr), nil
}
