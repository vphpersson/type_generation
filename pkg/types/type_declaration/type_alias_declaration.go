package type_declaration

import "reflect"

type TypeAliasDeclaration struct {
	Identifier string
	TypeParameters []string
	ReflectType reflect.Type
}

func (t *TypeAliasDeclaration) QualifiedName() string {
	return t.Identifier
}
