package tools

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/invopop/jsonschema"
)

func getJsonSchema[T any](targetStruct T) (map[string]any, error) {
	targetStructName, err := getStructName(targetStruct)
	if err != nil {
		return nil, err
	}
	params := jsonschema.Reflect(targetStruct)
	schema, ok := params.Definitions[targetStructName]
	if !ok {
		return nil, fmt.Errorf("could not find schema for %s", targetStructName)
	}
	return schemaToMap(schema)
}

func getStructName(targetStruct any) (string, error) {
	targetStructType := reflect.TypeOf(targetStruct)
	if targetStructType.Kind() == reflect.Pointer {
		targetStructType = targetStructType.Elem()
	}
	if targetStructType.Kind() != reflect.Struct {
		return "", fmt.Errorf("params target struct must be a struct or pointer to a struct, got %s", targetStructType.Kind())
	}
	return targetStructType.Name(), nil
}

func schemaToMap(schema *jsonschema.Schema) (map[string]any, error) {
	jsonSchema, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(jsonSchema, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
