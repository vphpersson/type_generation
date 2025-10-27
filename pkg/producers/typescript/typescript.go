package typescript

import (
	"fmt"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	"github.com/vphpersson/type_generation/pkg/producers/typescript/types"
	typeGenerationTypesContext "github.com/vphpersson/type_generation/pkg/types/context"
)

func Convert(values ...any) (string, error) {
	tsContext := types.Context{Context: typeGenerationTypesContext.New()}
	if err := tsContext.Add(values...); err != nil {
		return "", fmt.Errorf("add: %w", err)
	}

	output, err := tsContext.Render()
	if err != nil {
		return "", motmedelErrors.New(fmt.Errorf("render: %w", err), tsContext)
	}

	return output, nil
}
