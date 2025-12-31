package executor

import (
	"strings"

	"github.com/nghyane/llm-mux/internal/registry"
)

// ModelQuirks provides model-specific behavior detection.
// This centralizes checks that were previously scattered across the codebase.

// isClaudeModel returns true if the model name indicates a Claude model.
func isClaudeModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "claude")
}

// isGeminiModel returns true if the model name indicates a Gemini model.
func isGeminiModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.HasPrefix(lower, "gemini") || strings.Contains(lower, "gemini")
}

// isThinkingModel returns true if the model supports thinking/reasoning.
func isThinkingModel(model string) bool {
	info := registry.GetGlobalRegistry().GetModelInfo(model)
	return info != nil && info.Thinking != nil
}

// hasThinkingSuffix returns true if model name ends with "-thinking".
func hasThinkingSuffix(model string) bool {
	return strings.HasSuffix(model, "-thinking")
}

// getThinkingVariant returns the thinking variant of a model if it exists.
// Returns empty string if no thinking variant is available.
func getThinkingVariant(model string) string {
	if hasThinkingSuffix(model) {
		return model
	}
	thinkingModel := model + "-thinking"
	if registry.GetGlobalRegistry().GetModelInfo(thinkingModel) != nil {
		return thinkingModel
	}
	return ""
}

// getOutputTokenLimit returns the max output tokens for a model.
// Returns 0 if no limit is defined.
func getOutputTokenLimit(model string) int {
	info := registry.GetGlobalRegistry().GetModelInfo(model)
	if info == nil {
		return 0
	}
	if info.OutputTokenLimit > 0 {
		return info.OutputTokenLimit
	}
	return info.MaxCompletionTokens
}
