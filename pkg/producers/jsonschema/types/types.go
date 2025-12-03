package types

import (
    "encoding/json"
    "fmt"
    "reflect"
    "strings"

    motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
    motmedelReflect "github.com/Motmedel/utils_go/pkg/reflect"
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
func (c *Context) buildInterfaceSchema(iface *type_declaration.InterfaceDeclaration) (map[string]any, error) {
    schemaMap := map[string]any{
        "type": "object",
    }

    properties := map[string]any{}
    var required []string
    var additionalProps any = false

    for _, prop := range iface.Properties {
        if prop == nil {
            continue
        }
        field := prop.Field
        if field == nil {
            return nil, motmedelErrors.NewWithTrace(typeGenerationErrors.ErrNilField, prop)
        }

        identifier := prop.Identifier
        isOptional := prop.Optional

        jsonschemaTag, err := tag.New(field.Tag.Get("jsonschema"))
        if err != nil {
            return nil, fmt.Errorf("parse jsonschema tag: %w", err)
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
        }

        propSchema, err := c.GetJSONSchemaType(field.Type)
        if err != nil {
            return nil, fmt.Errorf("get json schema type (property): %w", err)
        }

        // Apply constraints from tag
        if jsonschemaTag != nil {
            // explicit format (overrides any inferred format, e.g., time.Time)
            if f := strings.TrimSpace(jsonschemaTag.Format); f != "" {
                propSchema["format"] = f
            }
            // string constraints
            if t, ok := propSchema["type"].(string); ok && t == "string" {
                if jsonschemaTag.MinLength > 0 {
                    propSchema["minLength"] = jsonschemaTag.MinLength
                }
                if jsonschemaTag.MaxLength > 0 {
                    propSchema["maxLength"] = jsonschemaTag.MaxLength
                }
            }
            // number/integer constraints
            if t, ok := propSchema["type"].(string); ok && (t == "number" || t == "integer") {
                if jsonschemaTag.Minimum != 0 {
                    propSchema["minimum"] = jsonschemaTag.Minimum
                }
                if jsonschemaTag.Maximum != 0 {
                    propSchema["maximum"] = jsonschemaTag.Maximum
                }
            }
        }

        properties[identifier] = propSchema
        if !isOptional {
            required = append(required, identifier)
        }
    }

    schemaMap["properties"] = properties
    if len(required) > 0 {
        schemaMap["required"] = required
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
    // Normalize root for container types to element struct type
    switch root.Kind() {
    case reflect.Map, reflect.Slice, reflect.Array:
        root = motmedelReflect.RemoveIndirection(root.Elem())
    }
    typeDecl, ok := c.TypeDeclarations[root]
    if !ok {
        return "", motmedelErrors.NewWithTrace(typeGenerationErrors.ErrUnsupportedKind, root.Kind())
    }
    rootIface, ok := typeDecl.(*type_declaration.InterfaceDeclaration)
    if !ok || rootIface == nil {
        return "", motmedelErrors.NewWithTrace(typeGenerationErrors.ErrUnsupportedKind, root.Kind())
    }

    // Build $defs for all interfaces
    defs := map[string]any{}
    for _, td := range c.TypeDeclarationsInOrder {
        iface, ok := td.(*type_declaration.InterfaceDeclaration)
        if !ok || iface == nil {
            continue
        }
        schema, err := c.buildInterfaceSchema(iface)
        if err != nil {
            return "", err
        }
        defs[iface.Identifier] = schema
    }

    top := map[string]any{
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "title":   rootIface.Identifier,
        "$defs":   defs,
    }
    // Reference the root schema via $defs to avoid duplicating the object at the top level
    top["$ref"] = "#/$defs/" + rootIface.Identifier

    b, err := json.MarshalIndent(top, "", "  ")
    if err != nil {
        return "", motmedelErrors.New(fmt.Errorf("json marshal indent: %w", err), top)
    }
    return string(b), nil
}
