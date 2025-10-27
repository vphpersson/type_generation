package generic_type_info

import "github.com/vphpersson/type_generation/pkg/types/shape"

type GenericTypeInfo struct {
	TypeParameterNames           []string
	FieldNameToShape             map[string]shape.Shape
	TypeParameterNameToFieldName map[string]string
}
