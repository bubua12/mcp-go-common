package mcputil

import "github.com/mark3labs/mcp-go/mcp"

// TextResult creates a successful tool result with text content.
func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: text},
		},
	}
}

// ErrorResult creates an error tool result (isError=true).
// The LLM can still read the error message and decide what to do next.
func ErrorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: msg},
		},
		IsError: true,
	}
}
