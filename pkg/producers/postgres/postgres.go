package postgres

import (
	"fmt"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	"github.com/vphpersson/type_generation/pkg/producers/postgres/types"
	typeGenerationTypesContext "github.com/vphpersson/type_generation/pkg/types/context"
)

func Convert(values ...any) (string, error) {
	postgresContext := types.Context{Context: typeGenerationTypesContext.New()}
	if err := postgresContext.Add(values...); err != nil {
		return "", fmt.Errorf("add: %w", err)
	}

	output, err := postgresContext.Render()
	if err != nil {
		return "", motmedelErrors.New(fmt.Errorf("render: %w", err), postgresContext)
	}

	return output, nil
}
