package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

const (
	OAuthTokenEnv  = "CLAUDE_CODE_OAUTH_TOKEN"
	localCacheDir  = ".yolopod"
	localCacheFile = "token"
)

func EnsureToken() (string, error) {
	// 1. Check env var
	if token := os.Getenv(OAuthTokenEnv); token != "" {
		return token, nil
	}

	// 2. Check cached token
	path, err := cachePath()
	if err != nil {
		return "", err
	}

	if data, err := os.ReadFile(path); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			fmt.Println("using cached claude auth token")
			return token, nil
		}
	}

	// 3. Prompt user to provide one (no echo)
	fmt.Println("no claude auth token found.")
	fmt.Println("run 'claude setup-token' in another terminal, then paste the token here.")
	fmt.Print("token: ")

	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after hidden input
	if err != nil {
		return "", fmt.Errorf("reading token: %w", err)
	}

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return "", fmt.Errorf("empty token provided")
	}

	// 4. Cache it
	if err := cacheToken(path, token); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not cache token: %v\n", err)
	}

	return token, nil
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, localCacheDir, localCacheFile), nil
}

func cacheToken(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0600)
}
