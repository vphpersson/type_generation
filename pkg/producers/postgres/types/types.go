package types

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	motmedelMaps "github.com/Motmedel/utils_go/pkg/maps"
	motmedelReflect "github.com/Motmedel/utils_go/pkg/reflect"
	"github.com/Motmedel/utils_go/pkg/utils"
	typeGenerationErrors "github.com/vphpersson/type_generation/pkg/errors"
	postgresErrors "github.com/vphpersson/type_generation/pkg/producers/postgres/errors"
	"github.com/vphpersson/type_generation/pkg/producers/postgres/types/tag"
	typeGenerationContext "github.com/vphpersson/type_generation/pkg/types/context"
	"github.com/vphpersson/type_generation/pkg/types/type_declaration"
)

// TODO: Move.

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(s string) string {
	s = matchFirstCap.ReplaceAllString(s, "${1}_${2}")
	s = matchAllCap.ReplaceAllString(s, "${1}_${2}")
	return strings.ToLower(s)
}

type AssociativeTable struct {
	SourceTableName string
	TargetTableName string
}

func (a *AssociativeTable) String() (string, error) {
	var propertyStrings []string
	for _, name := range []string{a.SourceTableName, a.TargetTableName} {
		propertyStrings = append(propertyStrings, fmt.Sprintf("\t%[1]s_id uuid NOT NULL REFERENCES %[1]s(id) ON DELETE CASCADE", name))
	}
	propertyStrings = append(propertyStrings, fmt.Sprintf("\tPRIMARY KEY (%s_id, %s_id)", a.SourceTableName, a.TargetTableName))

	return fmt.Sprintf(
		"CREATE TABLE %s (\n%s\n);",
		fmt.Sprintf("%s_%s", a.SourceTableName, a.TargetTableName),
		strings.Join(propertyStrings, ",\n"),
	), nil
}

func isTime(t reflect.Type) bool {
	return t.Name() == "Time" && t.PkgPath() == "time"
}

type Context struct {
	*typeGenerationContext.Context
}

func (c *Context) GetPostgresType(reflectType reflect.Type) (Type, error) {
	reflectType = motmedelReflect.RemoveIndirection(reflectType)

	var postgresType Type

	switch kind := reflectType.Kind(); kind {
	case reflect.Struct:
		if isTime(reflectType) {
			postgresType = Timestamp
		} else {
			typeDeclaration, err := motmedelMaps.MapGetNonZero(c.TypeDeclarations, reflectType)
			if err != nil {
				return nil, motmedelErrors.New(fmt.Errorf("map get non zero: %w", err), c.TypeDeclarations, reflectType)
			}

			interfaceDeclaration, err := utils.Convert[*type_declaration.InterfaceDeclaration](typeDeclaration)
			if err != nil {
				return nil, motmedelErrors.New(fmt.Errorf("convert: %w", err), typeDeclaration)
			}

			if genericTypeInfo := interfaceDeclaration.GenericTypeInfo; genericTypeInfo != nil {
				return nil, motmedelErrors.NewWithTrace(postgresErrors.ErrGenericTypesUnsupported)
			}

			postgresType = (&InterfaceDeclaration{InterfaceDeclaration: interfaceDeclaration, c: c}).TypeReference()
		}
	case reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16:
		postgresType = SmallInt
	case reflect.Int32, reflect.Uint32, reflect.Int, reflect.Uint:
		postgresType = Integer
	case reflect.Int64, reflect.Uint64:
		postgresType = BigInt
	case reflect.Float32:
		postgresType = Real
	case reflect.Float64:
		postgresType = DoublePrecision
	case reflect.String:
		postgresType = Text
	case reflect.Bool:
		postgresType = Boolean
	case reflect.Slice, reflect.Array:
		elemType := motmedelReflect.RemoveIndirection(reflectType.Elem())
		if elemType.Kind() == reflect.Uint8 {
			postgresType = ByteA
			break
		}

		itemPostgresType, err := c.GetPostgresType(elemType)
		if err != nil {
			return nil, motmedelErrors.New(fmt.Errorf("context get postgres type: %w", err), elemType)
		}

		if typeReference, ok := itemPostgresType.(*TypeReference); ok {
			postgresType = &AssociativeTable{TargetTableName: typeReference.TypeDeclaration.QualifiedName()}
		} else {
			postgresType = &ArrayType{ItemsType: itemPostgresType}
		}
	default:
		return nil, motmedelErrors.NewWithTrace(fmt.Errorf("%w: %T", typeGenerationErrors.ErrUnsupportedKind, kind), kind)
	}

	return postgresType, nil
}

func (c *Context) Render() (string, error) {
	var interfaceDeclarations []*InterfaceDeclaration

	for _, typeDeclaration := range c.TypeDeclarationsInOrder {
		switch v := any(typeDeclaration).(type) {
		case *type_declaration.InterfaceDeclaration:
			interfaceDeclarations = append(
				interfaceDeclarations,
				&InterfaceDeclaration{InterfaceDeclaration: v, c: c},
			)
		}
	}

	var stringBuilder strings.Builder

	for i, interfaceDeclaration := range interfaceDeclarations {
		if i > 0 {
			stringBuilder.WriteString("\n")
		}
		d, err := interfaceDeclaration.String()
		if err != nil {
			return "", motmedelErrors.New(fmt.Errorf("interface declaration string: %w", err), interfaceDeclaration)
		}
		stringBuilder.WriteString(d)
		stringBuilder.WriteString("\n")
	}

	return stringBuilder.String(), nil
}

type InterfaceDeclaration struct {
	*type_declaration.InterfaceDeclaration
	c *Context
}

func (t *InterfaceDeclaration) String() (string, error) {
	genericTypeInfo := t.GenericTypeInfo
	if genericTypeInfo != nil {
		return "", motmedelErrors.NewWithTrace(postgresErrors.ErrGenericTypesUnsupported)
	}

	var associativeTables []string
	var indices []string

	var propertyLines []string
	var uniqueCompositeFields []string
	var primaryKeyObserved bool

	for _, property := range t.Properties {
		if property == nil {
			continue
		}

		field := property.Field
		if field == nil {
			return "", motmedelErrors.NewWithTrace(typeGenerationErrors.ErrNilField, property)
		}

		fieldType := field.Type
		postgresType, err := t.c.GetPostgresType(fieldType)
		if err != nil {
			return "", motmedelErrors.New(fmt.Errorf("context get postgres type: %w", err), fieldType)
		}
		if utils.IsNil(postgresType) {
			return "", motmedelErrors.NewWithTrace(postgresErrors.ErrNilType)
		}

		if associativeTable, ok := postgresType.(*AssociativeTable); ok {
			associativeTable.SourceTableName = t.QualifiedName()
			tableString, err := associativeTable.String()
			if err != nil {
				return "", fmt.Errorf("type string: %w", err)
			}

			associativeTables = append(associativeTables, tableString)
			continue
		}

		typeString, err := postgresType.String()
		if err != nil {
			return "", fmt.Errorf("type string: %w", err)
		}

		identifier := property.Identifier
		var attributes []string
		optional := property.Optional

		postgresTag := tag.New(field.Tag.Get("postgres"))
		if postgresTag != nil {
			if postgresTag.Skip {
				continue
			}

			if name := postgresTag.Name; name != "" {
				identifier = name
			}

			if tagType := postgresTag.Type; tagType != "" {
				typeString = tagType
			}

			if postgresTag.Nullable {
				optional = true
			}

			if postgresTag.UniqueComposite {
				uniqueCompositeFields = append(uniqueCompositeFields, identifier)
			}

			if postgresTag.Indexed {
				indices = append(
					indices,
					fmt.Sprintf("CREATE INDEX %[1]s_%[2]s_idx ON %[1]s(%[2]s);", t.QualifiedName(), identifier),
				)
			}

			if postgresTag.PrimaryKey {
				primaryKeyObserved = true
				attributes = append(attributes, "PRIMARY KEY")
			}

			if _, ok := postgresType.(*TypeReference); ok {
				if onUpdate := postgresTag.OnUpdate; onUpdate != "" {
					attributes = append(attributes, fmt.Sprintf("ON UPDATE %s", onUpdate))
				}

				if onDelete := postgresTag.OnDelete; onDelete != "" {
					attributes = append(attributes, fmt.Sprintf("ON DELETE %s", onDelete))
				}
			}

			if postgresTag.Default != "" {
				attributes = append(attributes, fmt.Sprintf("DEFAULT %s", postgresTag.Default))
			}

			if unique := postgresTag.Unique; unique {
				attributes = append(attributes, "UNIQUE")
			}

			if generated := postgresTag.Generated; generated != "" {
				attributes = append(attributes, fmt.Sprintf("GENERATED ALWAYS AS (%s)", generated))
			}

			if generatedStored := postgresTag.GeneratedStored; generatedStored != "" {
				attributes = append(attributes, fmt.Sprintf("GENERATED ALWAYS AS (%s) STORED", generatedStored))
			}

			if check := postgresTag.Check; check != "" {
				attributes = append(attributes, fmt.Sprintf("CHECK (%s)", check))
			}
		}

		if !optional {
			attributes = append(attributes, "NOT NULL")
		}

		var attributesString string
		if len(attributes) > 0 {
			attributesString = fmt.Sprintf(" %s", strings.Join(attributes, " "))
		}

		propertyLines = append(
			propertyLines,
			fmt.Sprintf("\t%s %s%s", identifier, typeString, attributesString),
		)
	}

	if len(uniqueCompositeFields) > 0 {
		propertyLines = append(
			propertyLines,
			fmt.Sprintf("\tUNIQUE (%s)", strings.Join(uniqueCompositeFields, ", ")),
		)
	}

	if !primaryKeyObserved {
		propertyLines = append(
			propertyLines,
			"\tid uuid PRIMARY KEY DEFAULT gen_random_uuid()",
		)
	}

	table := fmt.Sprintf(
		"CREATE TABLE %s (\n%s\n);",
		t.QualifiedName(),
		strings.Join(propertyLines, ",\n"),
	)

	return strings.Join(slices.Concat([]string{table}, associativeTables, indices), "\n\n"), nil
}

func (t *InterfaceDeclaration) QualifiedName() string {
	return toSnakeCase(t.Identifier)
}

func (t *InterfaceDeclaration) TypeReference() *TypeReference {
	return &TypeReference{TypeDeclaration: t}
}
