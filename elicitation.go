package mcputil

import (
	"context"
	"errors"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DefaultConfirmTimeout is the default wait time for a user confirmation.
const DefaultConfirmTimeout = 120 * time.Second

// RequestConfirmation asks the human user (via the client UI, NOT the AI) to
// approve or reject an action, using MCP elicitation (spec 2025-06-18).
//
// Unlike a "confirm" tool exposed to the LLM, the confirmation request and
// response travel on a dedicated protocol channel (elicitation/create) that
// the AI cannot forge — the server handler blocks until the user answers.
//
// Returns:
//   - (true, nil)  user accepted
//   - (false, nil) user declined, cancelled, or the wait timed out
//   - (false, err) client does not support elicitation, or a transport error
//     occurred (callers should treat this as "cannot confirm" and refuse)
func RequestConfirmation(ctx context.Context, s *server.MCPServer, message string) (bool, error) {
	return RequestConfirmationWithTimeout(ctx, s, message, DefaultConfirmTimeout)
}

// RequestConfirmationWithTimeout is RequestConfirmation with a custom wait
// timeout. A timeout is treated as rejection (fail-closed).
func RequestConfirmationWithTimeout(ctx context.Context, s *server.MCPServer, message string, timeout time.Duration) (bool, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := s.RequestElicitation(ctx, mcp.ElicitationRequest{
		Params: mcp.ElicitationParams{
			Message: message,
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"confirm": map[string]any{
						"type":        "boolean",
						"description": "Approve this action",
					},
				},
				"required": []string{"confirm"},
			},
		},
	})
	if err != nil {
		// User did not answer in time: treat as rejection, not an error.
		if errors.Is(err, context.DeadlineExceeded) {
			return false, nil
		}
		return false, err
	}

	if result.Action != mcp.ElicitationResponseActionAccept {
		return false, nil
	}

	// Accept: honor the explicit confirm flag when present; a bare accept
	// (client closed the dialog with OK) counts as approval.
	if data, ok := result.Content.(map[string]any); ok {
		if confirm, ok := data["confirm"].(bool); ok {
			return confirm, nil
		}
	}
	return true, nil
}
