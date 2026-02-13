package login

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nghyane/llm-mux/internal/auth/claude"
	"github.com/nghyane/llm-mux/internal/browser"
	"github.com/nghyane/llm-mux/internal/config"
	log "github.com/nghyane/llm-mux/internal/logging"
	"github.com/nghyane/llm-mux/internal/misc"
	"github.com/nghyane/llm-mux/internal/provider"
)

type ClaudeAuthenticator struct {
}

func NewClaudeAuthenticator() *ClaudeAuthenticator {
	return &ClaudeAuthenticator{}
}

func (a *ClaudeAuthenticator) Provider() string {
	return "claude"
}

func (a *ClaudeAuthenticator) RefreshLead() *time.Duration {
	d := 4 * time.Hour
	return &d
}

func (a *ClaudeAuthenticator) Login(ctx context.Context, cfg *config.Config, opts *LoginOptions) (*provider.Auth, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cliproxy auth: configuration is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts == nil {
		opts = &LoginOptions{}
	}

	pkceCodes, err := claude.GeneratePKCECodes()
	if err != nil {
		return nil, fmt.Errorf("claude pkce generation failed: %w", err)
	}

	state, err := misc.GenerateRandomState()
	if err != nil {
		return nil, fmt.Errorf("claude state generation failed: %w", err)
	}

	authSvc := claude.NewClaudeAuth(cfg)

	authURL, returnedState, err := authSvc.GenerateAuthURL(state, pkceCodes)
	if err != nil {
		return nil, fmt.Errorf("claude authorisation url generation failed: %w", err)
	}
	state = returnedState

	if !opts.NoBrowser {
		if browser.IsAvailable() {
			fmt.Println("Opening browser for Claude authentication...")
			if err = browser.OpenURL(authURL); err != nil {
				log.Warnf("Failed to open browser automatically: %v", err)
			}
		}
	}

	fmt.Printf("Visit the following URL to authenticate:\n%s\n\n", authURL)
	fmt.Print("Paste the authorisation code from the browser: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read authorisation code: %w", err)
		}
		return nil, fmt.Errorf("no authorisation code provided")
	}
	code := strings.TrimSpace(scanner.Text())
	if code == "" {
		return nil, fmt.Errorf("empty authorisation code provided")
	}

	log.Debug("Claude authorisation code received; exchanging for tokens")

	authBundle, err := authSvc.ExchangeCodeForTokens(ctx, code, state, pkceCodes)
	if err != nil {
		return nil, claude.NewAuthenticationError(claude.ErrCodeExchangeFailed, err)
	}

	tokenStorage := authSvc.CreateTokenStorage(authBundle)

	if tokenStorage == nil || tokenStorage.Email == "" {
		return nil, fmt.Errorf("claude token storage missing account information")
	}

	fileName := fmt.Sprintf("claude-%s.json", tokenStorage.Email)
	metadata := map[string]any{
		"email": tokenStorage.Email,
	}

	if authBundle.APIKey != "" {
		fmt.Println("Claude API key obtained and stored")
	}

	return &provider.Auth{
		ID:       fileName,
		Provider: a.Provider(),
		FileName: fileName,
		Storage:  tokenStorage,
		Metadata: metadata,
	}, nil
}
