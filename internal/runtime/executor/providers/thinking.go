package providers

import (
	"github.com/nghyane/llm-mux/internal/registry"
	"github.com/nghyane/llm-mux/internal/translator/ir"
	"github.com/nghyane/llm-mux/internal/util"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ThinkingConfig encapsulates thinking/reasoning mode configuration for LLM requests.
// It provides a unified interface for applying provider-specific thinking configurations.
type ThinkingConfig struct {
	Enabled      bool
	BudgetTokens int
}

// ParseClaudeThinkingFromModel extracts thinking configuration from a Claude model name suffix.
// Uses single source of truth from registry via util.GetThinkingBudget.
// Returns nil if the model doesn't have a thinking suffix.
func ParseClaudeThinkingFromModel(modelName string) *ThinkingConfig {
	suffixLevel, isThinking := util.ParseThinkingSuffix(modelName)
	if !isThinking {
		return nil
	}

	budget, _ := util.GetThinkingBudget(modelName, suffixLevel, 0)
	if budget <= 0 {
		return nil
	}

	return &ThinkingConfig{
		Enabled:      true,
		BudgetTokens: budget,
	}
}

// ApplyToClaude applies thinking configuration to a Claude request body.
// Returns the modified body with thinking.type and thinking.budget_tokens set.
// If thinking config already exists in the body, returns the body unchanged.
func (t *ThinkingConfig) ApplyToClaude(body []byte) []byte {
	if t == nil || !t.Enabled {
		return body
	}
	if gjson.GetBytes(body, "thinking").Exists() {
		return body
	}
	body, _ = sjson.SetBytes(body, "thinking.type", "enabled")
	body, _ = sjson.SetBytes(body, "thinking.budget_tokens", t.BudgetTokens)
	return body
}

// EnsureClaudeMaxTokens ensures max_tokens is sufficient for thinking mode.
// Claude requires max_tokens >= budget_tokens + response_buffer when thinking is enabled.
func EnsureClaudeMaxTokens(modelName string, body []byte) []byte {
	thinkingType := gjson.GetBytes(body, "thinking.type").String()
	if thinkingType != "enabled" {
		return body
	}

	budgetTokens := gjson.GetBytes(body, "thinking.budget_tokens").Int()
	if budgetTokens <= 0 {
		return body
	}

	maxTokens := gjson.GetBytes(body, "max_tokens").Int()

	maxCompletionTokens := 0
	if modelInfo := registry.GetGlobalRegistry().GetModelInfo(modelName); modelInfo != nil {
		maxCompletionTokens = modelInfo.MaxCompletionTokens
	}

	const fallbackBuffer = 4000
	requiredMaxTokens := budgetTokens + fallbackBuffer
	if maxCompletionTokens > 0 {
		requiredMaxTokens = int64(maxCompletionTokens)
	}

	if maxTokens < requiredMaxTokens {
		body, _ = sjson.SetBytes(body, "max_tokens", requiredMaxTokens)
	}
	return body
}

func ApplyThinkingToIR(model string, req *ir.UnifiedChatRequest) {
	if req.Thinking != nil && req.Thinking.IncludeThoughts {
		return
	}

	cfg := ParseClaudeThinkingFromModel(model)
	if cfg == nil {
		return
	}

	budget := int32(cfg.BudgetTokens)
	req.Thinking = &ir.ThinkingConfig{
		IncludeThoughts: true,
		ThinkingBudget:  &budget,
	}

	info := registry.GetGlobalRegistry().GetModelInfo(model)
	minMaxTokens := cfg.BudgetTokens + 4000
	if info != nil && info.MaxCompletionTokens > 0 {
		minMaxTokens = info.MaxCompletionTokens
	}

	if req.MaxTokens == nil || *req.MaxTokens < minMaxTokens {
		mt := minMaxTokens
		req.MaxTokens = &mt
	}
}
