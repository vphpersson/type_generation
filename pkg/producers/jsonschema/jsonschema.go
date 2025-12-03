package jsonschema

import (
    "fmt"
    "reflect"

    motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
    "github.com/vphpersson/type_generation/pkg/producers/jsonschema/types"
    typeGenerationTypesContext "github.com/vphpersson/type_generation/pkg/types/context"
)

func Convert(root reflect.Type) (string, error) {
    jsonschemaContext := types.Context{Context: typeGenerationTypesContext.New()}
    if err := jsonschemaContext.Add(root); err != nil {
        return "", fmt.Errorf("add: %w", err)
    }

    output, err := jsonschemaContext.RenderRoot(root)
    if err != nil {
        return "", motmedelErrors.New(fmt.Errorf("render: %w", err), jsonschemaContext)
    }

    return output, nil
}
