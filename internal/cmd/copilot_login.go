package cmd

import (
	"context"
	"fmt"

	"github.com/nghyane/llm-mux/internal/auth/login"
	"github.com/nghyane/llm-mux/internal/config"
)

// DoCopilotLogin performs the GitHub Copilot OAuth device flow login.
func DoCopilotLogin(cfg *config.Config, options *LoginOptions) {
	if options == nil {
		options = &LoginOptions{}
	}

	manager := newAuthManager()

	authOpts := &login.LoginOptions{
		NoBrowser: options.NoBrowser,
		Metadata:  map[string]string{},
	}

	_, err := manager.Login(context.Background(), "github-copilot", cfg, authOpts)
	if err != nil {
		fmt.Printf("GitHub Copilot authentication failed: %v\n", err)
		return
	}

	fmt.Println("GitHub Copilot authentication successful!")
}
