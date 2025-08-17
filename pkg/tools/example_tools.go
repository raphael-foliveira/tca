package tools

import (
	"context"
	"os"
	"os/exec"
)

type GetFileTreeToolParams struct {
	Path string `json:"path" jsonschema:"default=./,description=The path to get the file tree for"`
}

func GetFileTreeTool() *Tool {
	return NewTool(
		"get_file_tree",
		"Get the file tree of the current directory",
		func(ctx context.Context, params GetFileTreeToolParams) (string, error) {
			if params.Path == "" {
				params.Path = "./"
			}
			result, err := exec.Command("tree", params.Path).Output()
			if err != nil {
				return "", err
			}
			return string(result), nil
		},
	)
}

type ReadFileToolParams struct {
	Path string `json:"path" jsonschema:"description=The path to read the file from"`
}

func ReadFileTool() *Tool {
	return NewTool(
		"read_file",
		"Read the contents of a file",
		func(ctx context.Context, params ReadFileToolParams) (string, error) {
			content, err := os.ReadFile(params.Path)
			if err != nil {
				return "", err
			}
			return string(content), nil
		},
	)
}
