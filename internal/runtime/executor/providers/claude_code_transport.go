package providers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const toolNamePrefix = "cc_"

var (
	prefixNeedle = []byte(`"name":"` + toolNamePrefix)
	prefixRepl   = []byte(`"name":"`)
)

type claudeCodeRoundTripper struct {
	base http.RoundTripper
}

func (t *claudeCodeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/v1/messages") {
		if req.Body != nil {
			body, err := io.ReadAll(req.Body)
			_ = req.Body.Close()
			if err != nil {
				return nil, err
			}

			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err == nil {
				prefixTools(payload)
				prefixToolChoice(payload)
				prefixToolNamesInMessages(payload)
				body, _ = json.Marshal(payload)
			}

			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		resp.Body = &toolNameStripReader{base: resp.Body}
	}
	return resp, nil
}

func prefixTools(payload map[string]any) {
	tools, ok := payload["tools"].([]any)
	if !ok {
		return
	}
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := tool["name"].(string); ok {
			tool["name"] = toolNamePrefix + name
		}
	}
}

func prefixToolChoice(payload map[string]any) {
	tc, ok := payload["tool_choice"].(map[string]any)
	if !ok {
		return
	}
	typ, _ := tc["type"].(string)
	if typ != "tool" {
		return
	}
	if name, ok := tc["name"].(string); ok {
		tc["name"] = toolNamePrefix + name
	}
}

func prefixToolNamesInMessages(payload map[string]any) {
	msgs, ok := payload["messages"].([]any)
	if !ok {
		return
	}
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, c := range content {
			block, ok := c.(map[string]any)
			if !ok {
				continue
			}
			typ, _ := block["type"].(string)
			if typ != "tool_use" && typ != "tool_result" {
				continue
			}
			if name, ok := block["name"].(string); ok {
				block["name"] = toolNamePrefix + name
			}
		}
	}
}

type toolNameStripReader struct {
	base io.ReadCloser
}

func (r *toolNameStripReader) Read(p []byte) (int, error) {
	n, err := r.base.Read(p)
	if n > 0 {
		replaced := bytes.ReplaceAll(p[:n], prefixNeedle, prefixRepl)
		n = copy(p, replaced)
	}
	return n, err
}

func (r *toolNameStripReader) Close() error {
	return r.base.Close()
}

func wrapClaudeCodeTransport(client *http.Client) {
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = &claudeCodeRoundTripper{base: base}
}
