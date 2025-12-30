// Package main demonstrates creating a custom AI provider executor
// and integrating it with the llm-mux server.
//
// This example uses httpbin.org as a demo upstream. Replace with your actual AI service.
//
// Note: Internal executors use BaseExecutor (internal/runtime/executor/base_executor.go)
// which provides helpers like NewHTTPClient, CountTokensNotSupported, etc.
// External providers implement the full interface manually as shown here.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nghyane/llm-mux/internal/api"
	"github.com/nghyane/llm-mux/internal/auth/login"
	"github.com/nghyane/llm-mux/internal/config"
	"github.com/nghyane/llm-mux/internal/logging"
	"github.com/nghyane/llm-mux/internal/provider"
	"github.com/nghyane/llm-mux/internal/service"
)

const providerKey = "myprov"

// notImplementedError implements clipexec.StatusError for 501 Not Implemented.
// Internal executors use NewNotImplementedError from internal/runtime/executor.
type notImplementedError struct{ msg string }

func (e notImplementedError) Error() string   { return e.msg }
func (e notImplementedError) StatusCode() int { return http.StatusNotImplemented }

// MyExecutor implements a minimal custom provider executor.
type MyExecutor struct{}

func (MyExecutor) Identifier() string { return providerKey }

// PrepareRequest injects credentials into the HTTP request.
func (MyExecutor) PrepareRequest(req *http.Request, a *provider.Auth) error {
	if req == nil || a == nil {
		return nil
	}
	if a.Attributes != nil {
		if ak := strings.TrimSpace(a.Attributes["api_key"]); ak != "" {
			req.Header.Set("Authorization", "Bearer "+ak)
		}
	}
	return nil
}

func buildHTTPClient(a *provider.Auth) *http.Client {
	if a == nil || strings.TrimSpace(a.ProxyURL) == "" {
		return http.DefaultClient
	}
	u, err := url.Parse(a.ProxyURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return http.DefaultClient
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
}

func upstreamEndpoint(a *provider.Auth) string {
	if a != nil && a.Attributes != nil {
		if ep := strings.TrimSpace(a.Attributes["endpoint"]); ep != "" {
			return ep
		}
	}
	// Demo echo endpoint; replace with your upstream.
	return "https://httpbin.org/post"
}

func (MyExecutor) Execute(ctx context.Context, a *provider.Auth, req provider.Request, opts provider.Options) (provider.Response, error) {
	client := buildHTTPClient(a)
	endpoint := upstreamEndpoint(a)

	httpReq, errNew := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(req.Payload))
	if errNew != nil {
		return provider.Response{}, errNew
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Inject credentials via PrepareRequest hook.
	_ = (MyExecutor{}).PrepareRequest(httpReq, a)

	resp, errDo := client.Do(httpReq)
	if errDo != nil {
		return provider.Response{}, errDo
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return provider.Response{Payload: body}, nil
}

func (MyExecutor) CountTokens(context.Context, *provider.Auth, provider.Request, provider.Options) (provider.Response, error) {
	return provider.Response{}, notImplementedError{"count tokens not supported for " + providerKey}
}

func (MyExecutor) ExecuteStream(_ context.Context, _ *provider.Auth, _ provider.Request, _ provider.Options) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 1)
	go func() {
		defer close(ch)
		ch <- provider.StreamChunk{Payload: []byte("data: {\"ok\":true}\n\n")}
	}()
	return ch, nil
}

func (MyExecutor) Refresh(_ context.Context, a *provider.Auth) (*provider.Auth, error) {
	return a, nil
}

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		panic(err)
	}

	tokenStore := login.GetTokenStore()
	if dirSetter, ok := tokenStore.(interface{ SetBaseDir(string) }); ok {
		dirSetter.SetBaseDir(cfg.AuthDir)
	}
	core := provider.NewManager(tokenStore, nil, nil)
	core.RegisterExecutor(MyExecutor{})

	hooks := service.Hooks{
		OnAfterStart: func(s *service.Service) {
			// Register demo models for the custom provider so they appear in /v1/models.
			models := []*service.ModelInfo{{ID: "myprov-pro-1", Object: "model", Type: providerKey, DisplayName: "MyProv Pro 1"}}
			for _, a := range core.List() {
				if strings.EqualFold(a.Provider, providerKey) {
					service.GlobalModelRegistry().RegisterClient(a.ID, providerKey, models)
				}
			}
		},
	}

	svc, err := service.NewBuilder().
		WithConfig(cfg).
		WithConfigPath("config.yaml").
		WithCoreAuthManager(core).
		WithServerOptions(
			// Optional: add a simple middleware + custom request logger
			api.WithMiddleware(func(c *gin.Context) { c.Header("X-Example", "custom-provider"); c.Next() }),
			api.WithRequestLoggerFactory(func(cfg *config.Config, cfgPath string) logging.RequestLogger {
				return logging.NewFileRequestLogger(true, "logs", filepath.Dir(cfgPath))
			}),
		).
		WithHooks(hooks).
		Build()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}
