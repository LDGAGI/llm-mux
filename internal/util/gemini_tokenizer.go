package util

import (
	"sync"

	"github.com/tidwall/gjson"
	"google.golang.org/genai"
	"google.golang.org/genai/tokenizer"
)

var (
	// tokenizerCache caches tokenizers by model name
	tokenizerCache   = make(map[string]*tokenizer.LocalTokenizer)
	tokenizerCacheMu sync.RWMutex
)

// getOrCreateTokenizer returns a cached tokenizer or creates a new one.
func getOrCreateTokenizer(model string) (*tokenizer.LocalTokenizer, error) {
	// Normalize model name for tokenizer (use base model)
	baseModel := normalizeModelForTokenizer(model)

	tokenizerCacheMu.RLock()
	tok, ok := tokenizerCache[baseModel]
	tokenizerCacheMu.RUnlock()
	if ok {
		return tok, nil
	}

	tokenizerCacheMu.Lock()
	defer tokenizerCacheMu.Unlock()

	// Double-check after acquiring write lock
	if tok, ok := tokenizerCache[baseModel]; ok {
		return tok, nil
	}

	tok, err := tokenizer.NewLocalTokenizer(baseModel)
	if err != nil {
		return nil, err
	}
	tokenizerCache[baseModel] = tok
	return tok, nil
}

// normalizeModelForTokenizer maps model names to tokenizer-compatible names.
func normalizeModelForTokenizer(model string) string {
	// Map specific models to their base tokenizer model
	switch {
	case containsAny(model, "gemini-3", "gemini-2.5", "gemini-2.0"):
		return "gemini-2.0-flash" // Use 2.0 tokenizer for newer models
	case containsAny(model, "gemini-1.5"):
		return "gemini-1.5-flash"
	case containsAny(model, "gemini-1.0", "gemini-pro"):
		return "gemini-1.0-pro"
	default:
		return "gemini-2.0-flash" // Default fallback
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// CountTokensFromGeminiRequest counts tokens from a Gemini API request payload.
// Returns the token count or 0 if counting fails (non-blocking).
func CountTokensFromGeminiRequest(model string, payload []byte) int64 {
	tok, err := getOrCreateTokenizer(model)
	if err != nil {
		return 0 // Fail silently, return 0
	}

	contents := extractContentsFromPayload(payload)
	if len(contents) == 0 {
		return 0
	}

	result, err := tok.CountTokens(contents, nil)
	if err != nil {
		return 0
	}

	return int64(result.TotalTokens)
}

// extractContentsFromPayload extracts genai.Content from Gemini request payload.
// Supports both standard Gemini format and Antigravity/GeminiCLI format (nested in "request").
func extractContentsFromPayload(payload []byte) []*genai.Content {
	var contents []*genai.Content

	// Check if contents is nested under "request" (Antigravity/GeminiCLI format)
	contentsPath := "contents"
	systemPath := "systemInstruction"
	if gjson.GetBytes(payload, "request.contents").Exists() {
		contentsPath = "request.contents"
		systemPath = "request.systemInstruction"
	}

	// Extract system instruction if present
	systemInstruction := gjson.GetBytes(payload, systemPath)
	if systemInstruction.Exists() {
		if content := parseContent(systemInstruction, "user"); content != nil {
			contents = append(contents, content)
		}
	}

	// Extract contents array
	contentsArr := gjson.GetBytes(payload, contentsPath)
	if !contentsArr.IsArray() {
		return contents
	}

	contentsArr.ForEach(func(_, value gjson.Result) bool {
		role := value.Get("role").String()
		if role == "" {
			role = "user"
		}
		if content := parseContent(value, role); content != nil {
			contents = append(contents, content)
		}
		return true
	})

	return contents
}

// parseContent parses a gjson.Result into genai.Content.
func parseContent(value gjson.Result, role string) *genai.Content {
	parts := value.Get("parts")
	if !parts.IsArray() {
		return nil
	}

	var genaiParts []*genai.Part
	parts.ForEach(func(_, part gjson.Result) bool {
		// Handle text parts
		if text := part.Get("text"); text.Exists() {
			genaiParts = append(genaiParts, genai.NewPartFromText(text.String()))
		}
		// Note: Images/audio would need different handling
		// For now, we only count text tokens (most accurate for context window)
		return true
	})

	if len(genaiParts) == 0 {
		return nil
	}

	return &genai.Content{
		Role:  role,
		Parts: genaiParts,
	}
}
