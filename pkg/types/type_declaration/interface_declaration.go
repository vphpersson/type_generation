package type_declaration

import (
	"reflect"

	"github.com/vphpersson/type_generation/pkg/types/generic_type_info"
)

type PropertySignature struct {
	Identifier string
	Field *reflect.StructField
	Optional   bool
}

type InterfaceDeclaration struct {
	Identifier      string
	Properties      []*PropertySignature
	GenericTypeInfo *generic_type_info.GenericTypeInfo
}

func (i *InterfaceDeclaration) QualifiedName() string {
	return i.Identifier
}
