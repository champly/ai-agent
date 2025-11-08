package agent

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ollama/ollama/api"
)

// MCPToolToOllamaTool 将 MCP Tool 转换为 Ollama Tool
func MCPToolToOllamaTool(mcpTool *mcp.Tool) api.Tool {
	tool := api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
		},
	}

	// 设置默认的 Parameters 类型
	tool.Function.Parameters.Type = "object"

	// 转换 InputSchema
	// https://github.com/google/jsonschema-go/blob/main/jsonschema/schema.go#L42
	if schema, ok := mcpTool.InputSchema.(map[string]any); ok {
		// 提取 required 字段
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					tool.Function.Parameters.Required = append(tool.Function.Parameters.Required, s)
				}
			}
		}

		// 转换 properties
		if props, ok := schema["properties"].(map[string]any); ok {
			tool.Function.Parameters.Properties = make(map[string]api.ToolProperty)

			for propName, propValue := range props {
				if propMap, ok := propValue.(map[string]any); ok {
					prop := api.ToolProperty{}

					// 提取 type
					if t, ok := propMap["type"].(string); ok {
						prop.Type = api.PropertyType{t}
					}

					// 提取 description
					if desc, ok := propMap["description"].(string); ok {
						prop.Description = desc
					}

					// 提取 enum（如果存在）
					if enum, ok := propMap["enum"].([]any); ok {
						prop.Enum = enum
					}

					tool.Function.Parameters.Properties[propName] = prop
				}
			}
		}
	}

	return tool
}
