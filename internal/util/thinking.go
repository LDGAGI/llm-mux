package util

import (
	"strings"

	"github.com/nghyane/llm-mux/internal/registry"
)

// DefaultThinkingBudget is the safe default budget for auto-enabling thinking.
// NOTE: For models with Thinking metadata, use GetAutoAppliedThinkingConfig which
// reads from registry for single source of truth. This constant is only a fallback.
const DefaultThinkingBudget = 1024

// DefaultThinkingBudgets provides fallback level-to-budget mapping when not
// defined in registry. These match common provider defaults.
var DefaultThinkingBudgets = registry.ThinkingBudgets{
	Low:    1024,
	Medium: 8192,
	High:   24576,
	Max:    32768,
}

// ThinkingLevel constants for suffix parsing.
type ThinkingLevel string

const (
	ThinkingLevelLow    ThinkingLevel = "low"
	ThinkingLevelMedium ThinkingLevel = "medium"
	ThinkingLevelHigh   ThinkingLevel = "high"
	ThinkingLevelMax    ThinkingLevel = "max"
)

// ModelSupportsThinking reports whether the given model has Thinking capability
// according to the model registry metadata (provider-agnostic).
func ModelSupportsThinking(model string) bool {
	if model == "" {
		return false
	}
	if info := registry.GetGlobalRegistry().GetModelInfo(model); info != nil {
		return info.Thinking != nil
	}
	return false
}

// GetModelThinkingMin returns the minimum thinking budget for a model
// from registry metadata. Returns 0 if model doesn't support thinking.
func GetModelThinkingMin(model string) int {
	if model == "" {
		return 0
	}
	if info := registry.GetGlobalRegistry().GetModelInfo(model); info != nil && info.Thinking != nil {
		return info.Thinking.Min
	}
	return 0
}

// GetDefaultThinkingBudget returns the appropriate default thinking budget for a model
// by reading from registry metadata. Uses model's Min as default if available,
// otherwise falls back to DefaultThinkingBudget.
func GetDefaultThinkingBudget(model string) int {
	if min := GetModelThinkingMin(model); min > 0 {
		// Use model's minimum as default (single source of truth)
		return min
	}
	return DefaultThinkingBudget
}

// GetAutoAppliedThinkingConfig returns the default thinking configuration for a model
// if it should be auto-applied. Returns (budget, include_thoughts, should_apply).
// Uses registry metadata for single source of truth on budget values.
func GetAutoAppliedThinkingConfig(model string) (int, bool, bool) {
	if ModelSupportsThinking(model) {
		budget := GetDefaultThinkingBudget(model)
		return budget, true, true
	}
	return 0, false, false
}

// ParseThinkingSuffix extracts thinking level from model name suffix.
// Returns (level, is_thinking_model).
func ParseThinkingSuffix(modelName string) (ThinkingLevel, bool) {
	switch {
	case strings.HasSuffix(modelName, "-thinking-max"):
		return ThinkingLevelMax, true
	case strings.HasSuffix(modelName, "-thinking-high"):
		return ThinkingLevelHigh, true
	case strings.HasSuffix(modelName, "-thinking-medium"):
		return ThinkingLevelMedium, true
	case strings.HasSuffix(modelName, "-thinking-low"):
		return ThinkingLevelLow, true
	case strings.HasSuffix(modelName, "-thinking"):
		// Default to max level for -thinking suffix
		return ThinkingLevelMax, true
	default:
		return "", false
	}
}

// GetThinkingBudget resolves the thinking budget for a model using single source of truth.
// Parameters:
//   - model: model name to resolve budget for
//   - suffixLevel: optional level from model name suffix (parsed from -thinking-*)
//   - userBudget: optional user-specified budget (0 means not specified)
//
// Returns (budget, includeThoughts, isThinking).
// Uses registry LevelBudgets, falls back to DefaultThinkingBudgets, then Min.
func GetThinkingBudget(model string, suffixLevel ThinkingLevel, userBudget int) (int, bool) {
	info := registry.GetGlobalRegistry().GetModelInfo(model)
	if info == nil || info.Thinking == nil {
		return 0, false
	}

	ts := info.Thinking
	var budget int

	// Priority 1: User-specified budget (if > 0)
	if userBudget > 0 {
		budget = userBudget
	} else if suffixLevel != "" {
		// Priority 2: Level from suffix, look up in registry LevelBudgets
		budgets := ts.Budgets
		if (budgets == registry.ThinkingBudgets{}) {
			// Fallback to default level budgets if not defined
			budgets = DefaultThinkingBudgets
		}

		switch suffixLevel {
		case ThinkingLevelLow:
			budget = budgets.Low
		case ThinkingLevelMedium:
			budget = budgets.Medium
		case ThinkingLevelHigh:
			budget = budgets.High
		case ThinkingLevelMax:
			budget = budgets.Max
		default:
			budget = 0
		}
	} else {
		// Priority 3: Default level from registry, or Min as fallback
		if ts.DefaultLevel != "" {
			budgets := ts.Budgets
			if (budgets == registry.ThinkingBudgets{}) {
				budgets = DefaultThinkingBudgets
			}
			switch ts.DefaultLevel {
			case registry.ThinkingLevelLow:
				budget = budgets.Low
			case registry.ThinkingLevelMedium:
				budget = budgets.Medium
			case registry.ThinkingLevelHigh:
				budget = budgets.High
			case registry.ThinkingLevelMax:
				budget = budgets.Max
			default:
				budget = ts.Min
			}
		} else {
			budget = ts.Min
		}
	}

	// Apply Min/Max clamping
	if budget < ts.Min && ts.Min > 0 {
		budget = ts.Min
	}
	if budget > ts.Max && ts.Max > 0 {
		budget = ts.Max
	}

	return budget, budget > 0
}
