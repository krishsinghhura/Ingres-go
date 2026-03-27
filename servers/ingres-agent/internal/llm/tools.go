package llm

import (
	"encoding/json"
	"fmt"
)

type ToolDefinition struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Parameters  json.RawMessage `json:"parameters,omitempty"`
	} `json:"function"`
}

func GetToolsSchema() []ToolDefinition {
	calculateParams := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "The math expression to evaluate (e.g., '25 * 4')",
			},
		},
		"required": []string{"expression"},
	}
	paramsJSON, _ := json.Marshal(calculateParams)

	return []ToolDefinition{
		{
			Type: "function",
			Function: struct {
				Name        string          `json:"name"`
				Description string          `json:"description,omitempty"`
				Parameters  json.RawMessage `json:"parameters,omitempty"`
			}{
				Name:        "calculate",
				Description: "Evaluate a mathematical expression",
				Parameters:  paramsJSON,
			},
		},
	}
}

func ExecuteTool(name string, arguments string) (string, error) {
	switch name {
	case "calculate":
		var args struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", err
		}
		return fmt.Sprintf("Result of %s is evaluated internally.", args.Expression), nil
	default:
		return "", fmt.Errorf("tool not found: %s", name)
	}
}
