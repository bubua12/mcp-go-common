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

// ConfirmOutcome is the result of a user confirmation request.
type ConfirmOutcome int

const (
	// ConfirmApproved — user explicitly approved.
	ConfirmApproved ConfirmOutcome = iota
	// ConfirmDeclined — user explicitly declined.
	ConfirmDeclined
	// ConfirmCancelled — the request was cancelled: user dismissed the dialog,
	// or the client has no elicitation UI and silently cancels (known behavior
	// of Claude Desktop and the Claude Code VS Code extension).
	ConfirmCancelled
	// ConfirmTimedOut — no answer within the wait timeout (fail-closed).
	ConfirmTimedOut
)

// String returns a human-readable name for the outcome.
func (o ConfirmOutcome) String() string {
	switch o {
	case ConfirmApproved:
		return "approved"
	case ConfirmDeclined:
		return "declined"
	case ConfirmCancelled:
		return "cancelled"
	case ConfirmTimedOut:
		return "timeout"
	default:
		return "unknown"
	}
}

// RequestConfirmation asks the human user (via the client UI, NOT the AI) to
// approve or reject an action, using MCP elicitation (spec 2025-06-18).
//
// Unlike a "confirm" tool exposed to the LLM, the confirmation request and
// response travel on a dedicated protocol channel (elicitation/create) that
// the AI cannot forge — the server handler blocks until the user answers.
//
// Returns the outcome (approved/declined/cancelled/timeout) so callers can
// message each case accurately, plus an error which is non-nil only when the
// request itself failed (client does not support elicitation, transport
// error). Callers should treat any non-approved outcome as "do not proceed".
func RequestConfirmation(ctx context.Context, s *server.MCPServer, message string) (ConfirmOutcome, error) {
	return RequestConfirmationWithTimeout(ctx, s, message, DefaultConfirmTimeout)
}

// RequestConfirmationWithTimeout is RequestConfirmation with a custom wait
// timeout. A timeout is reported as ConfirmTimedOut, not an error.
func RequestConfirmationWithTimeout(ctx context.Context, s *server.MCPServer, message string, timeout time.Duration) (ConfirmOutcome, error) {
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
		// User did not answer in time: fail-closed, but not an error.
		if errors.Is(err, context.DeadlineExceeded) {
			return ConfirmTimedOut, nil
		}
		return ConfirmCancelled, err
	}

	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		// Honor the explicit confirm flag when present; a bare accept
		// (client closed the dialog with OK) counts as approval.
		if data, ok := result.Content.(map[string]any); ok {
			if confirm, ok := data["confirm"].(bool); ok && !confirm {
				return ConfirmDeclined, nil
			}
		}
		return ConfirmApproved, nil
	case mcp.ElicitationResponseActionDecline:
		return ConfirmDeclined, nil
	default: // cancel and any future actions
		return ConfirmCancelled, nil
	}
}
