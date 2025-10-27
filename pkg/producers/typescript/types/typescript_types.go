package types

import (
	"fmt"
	"strings"
)

type Type interface {
	ToTypeScript() string
}

type TypeDeclaration interface {
	TypeReference() *TypeReference
	QualifiedName() string
}

type TypeReference struct {
	TypeDeclaration TypeDeclaration
	TypeArguments   []Type
}

func (t *TypeReference) ToTypeScript() string {
	name := t.TypeDeclaration.QualifiedName()
	if len(t.TypeArguments) == 0 {
		return name
	}

	args := make([]string, 0, len(t.TypeArguments))
	for _, a := range t.TypeArguments {
		args = append(args, a.ToTypeScript())
	}

	return fmt.Sprintf("%s<%s>", name, strings.Join(args, ", "))
}

type TypeParameter struct {
	Identifier string
}

func (p *TypeParameter) ToTypeScript() string { return p.Identifier }

type BasicType string

const (
	Boolean = BasicType("boolean")
	Number  = BasicType("number")
	String  = BasicType("string")
	Null    = BasicType("null")
	Any     = BasicType("any")
)

func (b BasicType) ToTypeScript() string { return string(b) }

type UnionType struct {
	Types []Type
}

func (u UnionType) ToTypeScript() string {
	var tsTypes []string
	for _, t := range u.Types {
		tsTypes = append(tsTypes, t.ToTypeScript())
	}
	return strings.Join(tsTypes, " | ")
}

type MapType struct {
	IndexType Type
	ValueType Type
}

// ToTypeScript implements the Type interface.
func (m *MapType) ToTypeScript() string {
	indexTypeToTS := m.IndexType.ToTypeScript()
	if indexTypeToTS != "number" && indexTypeToTS != "string" {
		panic(fmt.Sprintf("TypeScript type %q cannot be used as an index signature parameter type.", indexTypeToTS))
	}

	return fmt.Sprintf("{ [key: %s]: %s }", indexTypeToTS, m.ValueType.ToTypeScript())
}

type ArrayType struct {
	ItemsType Type
}

// ToTypeScript implements the Type interface.
func (a *ArrayType) ToTypeScript() string {
	fmtStr := "%s[]"
	if _, ok := a.ItemsType.(*UnionType); ok {
		fmtStr = "(%s)[]"
	}
	return fmt.Sprintf(fmtStr, a.ItemsType.ToTypeScript())
}
