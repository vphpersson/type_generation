package context

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/vphpersson/type_generation/internal/generic_type_info"
	typeGeneratorErrors "github.com/vphpersson/type_generation/pkg/errors"
	"github.com/vphpersson/type_generation/pkg/types/shape"
	"github.com/vphpersson/type_generation/pkg/types/type_declaration"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	motmedelJsonTag "github.com/Motmedel/utils_go/pkg/json/types/tag"
	motmedelReflect "github.com/Motmedel/utils_go/pkg/reflect"
)

var nonNumberPrimitiveKinds = map[reflect.Kind]bool{
	reflect.Bool:    true,
	reflect.Uintptr: true,
	reflect.String:  true,
}

var numberKinds = map[reflect.Kind]bool{
	reflect.Int:     true,
	reflect.Int8:    true,
	reflect.Int16:   true,
	reflect.Int32:   true,
	reflect.Int64:   true,
	reflect.Uint:    true,
	reflect.Uint8:   true,
	reflect.Uint16:  true,
	reflect.Uint32:  true,
	reflect.Uint64:  true,
	reflect.Float32: true,
	reflect.Float64: true,
}

func isPrimitive(kind reflect.Kind) bool {
	return numberKinds[kind] || nonNumberPrimitiveKinds[kind]
}

func isPrimitiveAlias(reflectType reflect.Type) bool {
	return isPrimitive(reflectType.Kind()) && reflectType.Name() != reflectType.Kind().String()
}

type optionalFieldPolicy int

const (
	forceOptional optionalFieldPolicy = iota
	dontForceOptional
)

type Context struct {
	TypeDeclarations        map[reflect.Type]type_declaration.TypeDeclaration
	TypeDeclarationsInOrder []type_declaration.TypeDeclaration

	usedQualifiedNames map[string]struct{}
	anonymousCount     int
}

func (g *Context) makeUniqueIdentifier(base string) string {
	id := base
	i := 2
	for {
		if _, exists := g.usedQualifiedNames[id]; !exists {
			return id
		}
		id = fmt.Sprintf("%s%d", base, i)
		i++
	}
}

func (g *Context) makeUniqueAnonymousIdentifier() string {
	g.anonymousCount++
	return fmt.Sprintf("Anonymous%d", g.anonymousCount)
}

func (g *Context) populateProperties(
	interfaceDeclaration *type_declaration.InterfaceDeclaration,
	structType reflect.Type,
	optionalFieldPolicy optionalFieldPolicy,
) error {
	structType = motmedelReflect.RemoveIndirection(structType)
	// TODO: Should I check structType.Kind() == reflect.Struct?

	// Iterate over normal fields first, and embedded structs last. This ensures that outer fields
	// will take precedence over inner fields in the case of overlapping fields, which is consistent
	// with json.Marshal().
	var embeddedFields []reflect.StructField
	for i := range structType.NumField() {
		field := structType.Field(i)

		if len(field.Name) == 0 || !ast.IsExported(field.Name) {
			continue
		}

		if field.Anonymous && motmedelReflect.RemoveIndirection(field.Type).Kind() == reflect.Struct {
			embeddedFields = append(embeddedFields, field)
			continue
		}

		// NOTE: Maybe this should not be here. Maybe it should be up to the output generator what struct tag to use.

		propertyName := field.Name
		optional := false

		jsonTag := motmedelJsonTag.New(field.Tag.Get("json"))
		if jsonTag != nil {
			if jsonTag.Skip {
				continue
			}
			if name := jsonTag.Name; name != "" {
				propertyName = name
			}

			optional = jsonTag.OmitEmpty || jsonTag.OmitZero
		}

		// If there's already a field with the same name as the current field, we skip it. This can
		// happen when populating the fields of an embedded struct, and the inner and outer structs
		// have overlapping fields, in which case the outer field takes precedence.
		overlaps := false
		for _, property := range interfaceDeclaration.Properties {
			if propertyName == property.Identifier {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		directType := motmedelReflect.RemoveIndirection(field.Type)

		switch directType.Kind() {
		case reflect.Struct:
			if _, err := g.GetOrCreateInterfaceDeclaration(directType); err != nil {
				return motmedelErrors.New(
					fmt.Errorf("get or create interface declaration: %w", err),
					directType,
				)
			}
		case reflect.Map, reflect.Slice, reflect.Array:
			directTypeElem := motmedelReflect.RemoveIndirection(directType.Elem())
			if directTypeElem.Kind() == reflect.Struct {
				if _, err := g.GetOrCreateInterfaceDeclaration(directTypeElem); err != nil {
					return motmedelErrors.New(
						fmt.Errorf("get or create interface declaration: %w", err),
						directTypeElem,
					)
				}
			}
		default:
		}

		useTypeAlias := directType.Name() != "" && (!isPrimitive(directType.Kind()) || isPrimitiveAlias(directType))

		if useTypeAlias {
			if _, ok := g.TypeDeclarations[directType]; !ok {
				typeName, _ := motmedelReflect.GetTypeName(directType)
				identifier := strings.Title(typeName)
				if identifier == "" {
					identifier = g.makeUniqueAnonymousIdentifier()
				}

				uniqueInterfaceName := g.makeUniqueIdentifier(identifier)
				g.usedQualifiedNames[uniqueInterfaceName] = struct{}{}

				typeDeclaration := &type_declaration.TypeAliasDeclaration{
					Identifier:  identifier,
					ReflectType: directType,
				}
				g.TypeDeclarations[directType] = typeDeclaration
			}
		}

		interfaceDeclaration.Properties = append(
			interfaceDeclaration.Properties,
			&type_declaration.PropertySignature{
				Identifier: propertyName,
				Field:      &field,
				Optional:   optionalFieldPolicy == forceOptional || optional,
			},
		)
	}

	for _, field := range embeddedFields {
		// If the field is an embedded struct pointer, we recursively mark all its fields as optional.
		// This is because json.Marshal() will omit said fields if the embedded struct pointer is nil.
		embeddedStructOptionalFieldPolicy := dontForceOptional
		if optionalFieldPolicy == forceOptional || field.Type.Kind() == reflect.Ptr {
			embeddedStructOptionalFieldPolicy = forceOptional
		}

		err := g.populateProperties(interfaceDeclaration, field.Type, embeddedStructOptionalFieldPolicy)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Context) GetOrCreateInterfaceDeclaration(structType reflect.Type) (*type_declaration.InterfaceDeclaration, error) {
	structType = motmedelReflect.RemoveIndirection(structType)
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("cannot declare interface for type %q", structType.Kind())
	}

	if existingTypeDeclaration, ok := g.TypeDeclarations[structType]; ok {
		return existingTypeDeclaration.(*type_declaration.InterfaceDeclaration), nil
	}

	typeName, isGenericType := motmedelReflect.GetTypeName(structType)
	interfaceName := strings.Title(typeName)
	if interfaceName == "" {
		interfaceName = g.makeUniqueAnonymousIdentifier()
	}

	uniqueInterfaceName := g.makeUniqueIdentifier(interfaceName)
	g.usedQualifiedNames[uniqueInterfaceName] = struct{}{}

	interfaceDeclaration := &type_declaration.InterfaceDeclaration{Identifier: uniqueInterfaceName}
	g.TypeDeclarations[structType] = interfaceDeclaration

	if isGenericType {
		// Use AST to discover generic type info.

		genericTypeInfo, err := generic_type_info.GetGenericTypeInfo(structType)
		if err != nil {
			return nil, fmt.Errorf("get generic type info: %w", err)
		}
		if genericTypeInfo == nil {
			return nil, motmedelErrors.NewWithTrace(typeGeneratorErrors.ErrNilGenericTypeInfo)
		}

		interfaceDeclaration.GenericTypeInfo = genericTypeInfo
		interfaceDeclaration.TypeParameters = genericTypeInfo.TypeParameterNames

		for _, typeParameterName := range genericTypeInfo.TypeParameterNames {
			fieldName := genericTypeInfo.TypeParameterNameToFieldName[typeParameterName]
			if fieldName == "" {
				continue
			}

			field, ok := structType.FieldByName(fieldName)
			if !ok {
				continue
			}

			argReflectType := field.Type
			if fieldShape, ok := genericTypeInfo.FieldNameToShape[fieldName]; ok {
				switch fieldShape.Kind {
				case shape.KindPointer:
					argReflectType = motmedelReflect.RemoveIndirection(argReflectType)
				case shape.KindSlice, shape.KindArray:
					argReflectType = argReflectType.Elem()
				case shape.KindMapValue:
					argReflectType = argReflectType.Elem()
				case shape.KindMapKey:
					argReflectType = argReflectType.Key()
				case shape.KindDirect:
					// use as-is
				}
			}

			if directType := motmedelReflect.RemoveIndirection(argReflectType); directType.Kind() == reflect.Struct {
				if _, err = g.GetOrCreateInterfaceDeclaration(directType); err != nil {
					return nil, motmedelErrors.New(
						fmt.Errorf("get or create interface declaration: %w", err),
						directType,
					)
				}
			}
		}
	}

	if err := g.populateProperties(interfaceDeclaration, structType, dontForceOptional); err != nil {
		return nil, fmt.Errorf("populate interface declaration properties: %w", err)
	}

	// Add the interface declaration to the ordered output after populating its fields. This ensures
	// that any new types discovered while populating the interface fields will appear before the
	// interface declaration in the output TypeScript code.
	g.TypeDeclarationsInOrder = append(g.TypeDeclarationsInOrder, interfaceDeclaration)

	return interfaceDeclaration, nil
}

func (g *Context) Add(values ...any) error {
	for _, value := range values {
		var reflectType reflect.Type
		switch v := value.(type) {
		case reflect.Type:
			reflectType = v
		case reflect.Value:
			reflectType = v.Type()
		default:
			reflectType = reflect.TypeOf(v)
		}

		if _, err := g.GetOrCreateInterfaceDeclaration(reflectType); err != nil {
			return fmt.Errorf("get or create interface declaration: %w", err)
		}
	}

	return nil
}

func New() *Context {
	return &Context{
		TypeDeclarations:        map[reflect.Type]type_declaration.TypeDeclaration{},
		TypeDeclarationsInOrder: []type_declaration.TypeDeclaration{},
		usedQualifiedNames:      map[string]struct{}{},
	}
}
