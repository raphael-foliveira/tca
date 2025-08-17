package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/raphael-foliveira/tca/pkg/utils"
)

type FunctionDefinitionParam struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitzero"`
	Parameters  map[string]any `json:"parameters,omitzero"`
}

type Tool struct {
	name        string
	description string
	params      any
	execute     func(ctx context.Context, params string) (string, error)

	functionDefinition *FunctionDefinitionParam
	once               sync.Once
}

func NewTool[T any](
	name, description string,
	run func(ctx context.Context, params T) (string, error),
) *Tool {
	var zeroT T
	return &Tool{
		name:        name,
		description: description,
		params:      zeroT,
		execute: func(ctx context.Context, params string) (string, error) {
			var paramsT T
			err := json.Unmarshal([]byte(params), &paramsT)
			if err != nil {
				return "", fmt.Errorf("error unmarshalling params: %w, params: %s", err, params)
			}
			return run(ctx, paramsT)
		},
	}
}

func (t *Tool) ToFunctionDefinition() (FunctionDefinitionParam, error) {
	schema, err := utils.GetJsonSchema(t.params)
	if err != nil {
		return FunctionDefinitionParam{}, err
	}
	return FunctionDefinitionParam{
		Name:        t.name,
		Description: t.description,
		Parameters:  schema,
	}, nil
}

func (t *Tool) GetDefinition() (FunctionDefinitionParam, error) {
	var errDefinition error
	t.once.Do(func() {
		definition, err := t.ToFunctionDefinition()
		if err != nil {
			errDefinition = err
		}
		t.functionDefinition = &definition
	})
	if errDefinition != nil {
		return FunctionDefinitionParam{}, errDefinition
	}
	return *t.functionDefinition, nil
}

func (t *Tool) GetName() string {
	return t.name
}

func (t *Tool) Run(ctx context.Context, params string) (string, error) {
	return t.execute(ctx, params)
}
