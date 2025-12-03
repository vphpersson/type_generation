package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	motmedelReflect "github.com/Motmedel/utils_go/pkg/reflect"
	"github.com/Motmedel/utils_go/pkg/utils"
	typeGenerationErrors "github.com/vphpersson/type_generation/pkg/errors"
	"github.com/vphpersson/type_generation/pkg/producers/jsonschema/types/tag"
	typeGenerationContext "github.com/vphpersson/type_generation/pkg/types/context"
	"github.com/vphpersson/type_generation/pkg/types/type_declaration"
)

type Context struct {
	*typeGenerationContext.Context
}

func isTime(t reflect.Type) bool {
	return t.Name() == "Time" && t.PkgPath() == "time"
}

// GetJSONSchemaType returns a JSON Schema fragment describing the provided type.
func (c *Context) GetJSONSchemaType(reflectType reflect.Type) (map[string]any, error) {
	reflectType = motmedelReflect.RemoveIndirection(reflectType)

	switch kind := reflectType.Kind(); kind {
	case reflect.Struct:
		if isTime(reflectType) {
			return map[string]any{"type": "string", "format": "date-time"}, nil
		}

		// Reference another interface via local $defs
		typeDeclaration, ok := c.TypeDeclarations[reflectType]
		if ok {
			if iface, ok2 := typeDeclaration.(*type_declaration.InterfaceDeclaration); ok2 {
				return map[string]any{"$ref": "#/$defs/" + iface.QualifiedName()}, nil
			}
		}
		return nil, motmedelErrors.NewWithTrace(typeGenerationErrors.ErrUnsupportedKind, kind)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Slice, reflect.Array:
		// Special case: []byte -> base64 string
		elem := motmedelReflect.RemoveIndirection(reflectType.Elem())
		if elem.Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}, nil
		}
		itemSchema, err := c.GetJSONSchemaType(elem)
		if err != nil {
			return nil, motmedelErrors.New(fmt.Errorf("get json schema type (items): %w", err), elem)
		}
		return map[string]any{"type": "array", "items": itemSchema}, nil
	case reflect.Map:
		// JSON object with additionalProperties as value schema
		value := motmedelReflect.RemoveIndirection(reflectType.Elem())
		valueSchema, err := c.GetJSONSchemaType(value)
		if err != nil {
			return nil, motmedelErrors.New(fmt.Errorf("get json schema type (map value): %w", err), value)
		}
		return map[string]any{"type": "object", "additionalProperties": valueSchema}, nil
	case reflect.Pointer:
		return c.GetJSONSchemaType(reflectType.Elem())
	default:
		return nil, motmedelErrors.NewWithTrace(
			fmt.Errorf("%w: %T", typeGenerationErrors.ErrUnsupportedKind, kind), kind,
		)
	}
}

// buildInterfaceSchema builds the object schema for a given interface declaration
func (c *Context) buildInterfaceSchema(interfaceDeclaration *type_declaration.InterfaceDeclaration) (map[string]any, error) {
	schemaMap := map[string]any{
		"type": "object",
	}

	properties := map[string]any{}
	var requiredProperties []string
	// TODO: Should this be anything other than false? Control with a `_` field?
	var additionalProps any = false

	for _, property := range interfaceDeclaration.Properties {
		if property == nil {
			continue
		}

		field := property.Field
		if field == nil {
			return nil, motmedelErrors.NewWithTrace(typeGenerationErrors.ErrNilField, property)
		}

		identifier := property.Identifier
		isOptional := property.Optional

		rawJsonSchemaTag := field.Tag.Get("jsonschema")
		jsonschemaTag, err := tag.New(rawJsonSchemaTag)
		if err != nil {
			return nil, motmedelErrors.New(fmt.Errorf("jsonschema tag new: %w", err), rawJsonSchemaTag)
		}

		if jsonschemaTag != nil {
			if jsonschemaTag.Skip {
				continue
			}

			if name := strings.TrimSpace(jsonschemaTag.Name); name != "" {
				identifier = name
			}

			if jsonschemaTag.Optional {
				isOptional = true
			}
		} else {
			// As a fallback, if there is a `json` tag with a name, use that name as the identifier.
			jsonTagRaw := field.Tag.Get("json")
			jsonTag, err := tag.New(jsonTagRaw)
			if err != nil {
				return nil, motmedelErrors.New(fmt.Errorf("json tag new: %w", err), jsonTagRaw)
			}
			if jsonTag != nil {
				if name := strings.TrimSpace(jsonTag.Name); name != "" {
					identifier = name
				}
			}
		}

		fieldType := field.Type
		propertySchema, err := c.GetJSONSchemaType(fieldType)
		if err != nil {
			return nil, motmedelErrors.New(fmt.Errorf("get json schema type: %w", err), fieldType)
		}

		// Apply constraints from the jsonschema tag.
		if jsonschemaTag != nil {
			// explicit format (overrides any inferred format, e.g., time.Time)
			if f := strings.TrimSpace(jsonschemaTag.Format); f != "" {
				propertySchema["format"] = f
			}
			// string constraints
			if t, ok := propertySchema["type"].(string); ok && t == "string" {
				if jsonschemaTag.MinLength > 0 {
					propertySchema["minLength"] = jsonschemaTag.MinLength
				}
				if jsonschemaTag.MaxLength > 0 {
					propertySchema["maxLength"] = jsonschemaTag.MaxLength
				}
			}
			// number/integer constraints
			if t, ok := propertySchema["type"].(string); ok && (t == "number" || t == "integer") {
				if jsonschemaTag.Minimum != 0 {
					propertySchema["minimum"] = jsonschemaTag.Minimum
				}
				if jsonschemaTag.Maximum != 0 {
					propertySchema["maximum"] = jsonschemaTag.Maximum
				}
			}
		}

		properties[identifier] = propertySchema
		if !isOptional {
			requiredProperties = append(requiredProperties, identifier)
		}
	}

	schemaMap["properties"] = properties
	if len(requiredProperties) > 0 {
		schemaMap["required"] = requiredProperties
	} else {
		schemaMap["required"] = []string{}
	}

	schemaMap["additionalProperties"] = additionalProps

	return schemaMap, nil
}

// RenderRoot builds a single JSON Schema document with the provided root type as the top-level schema
// and all discovered interfaces included under $defs. References use local $refs to $defs.
func (c *Context) RenderRoot(root reflect.Type) (string, error) {
	root = motmedelReflect.RemoveIndirection(root)

	rootKind := root.Kind()
	if rootKind != reflect.Struct {
		return "", motmedelErrors.NewWithTrace(typeGenerationErrors.ErrUnsupportedKind, rootKind)
	}

	rootTypeDeclaration, ok := c.TypeDeclarations[root]
	if !ok {
		return "", motmedelErrors.NewWithTrace(
			fmt.Errorf("%w (root type)", motmedelErrors.ErrNotInMap),
			root,
		)
	}

	rootInterfaceDeclaration, err := utils.ConvertToNonZero[*type_declaration.InterfaceDeclaration](rootTypeDeclaration)
	if err != nil {
		return "", motmedelErrors.New(
			fmt.Errorf("convert to non zero (root type declaration): %w", err),
			rootTypeDeclaration,
		)
	}

	// Build $defs for all interfaces
	defs := map[string]any{}
	for _, typeDeclaration := range c.TypeDeclarationsInOrder {
		interfaceDeclaration, ok := typeDeclaration.(*type_declaration.InterfaceDeclaration)
		if !ok || interfaceDeclaration == nil {
			continue
		}

		schema, err := c.buildInterfaceSchema(interfaceDeclaration)
		if err != nil {
			return "", motmedelErrors.New(fmt.Errorf("build interface schema: %w", err), interfaceDeclaration)
		}

		defs[interfaceDeclaration.Identifier] = schema
	}

	rootInterfaceDeclarationIdentifier := rootInterfaceDeclaration.Identifier

	schemaMap := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title":   rootInterfaceDeclarationIdentifier,
		"$defs":   defs,
	}
	// Reference the root schema via $defs to avoid duplicating the object at the top level
	schemaMap["$ref"] = "#/$defs/" + rootInterfaceDeclarationIdentifier

	data, err := json.Marshal(schemaMap)
	if err != nil {
		return "", motmedelErrors.NewWithTrace(fmt.Errorf("json marshal (schema map): %w", err), schemaMap)
	}

	return string(data), nil
}
