package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/007/yolopod/internal/auth"
	"github.com/007/yolopod/internal/config"
	"github.com/007/yolopod/internal/pod"
	yolosync "github.com/007/yolopod/internal/sync"
)

func main() {
	syncBack := flag.Bool("sync-back", false, "sync git changes back to local workspace after session ends")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: yolopod [flags] <config.toml>\n\nflags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := run(cfg, *syncBack); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, syncBack bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel context on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\nreceived signal, cleaning up...\n")
		cancel()
	}()

	// Ensure we have a Claude auth token before creating the pod
	token, err := auth.EnsureToken()
	if err != nil {
		return fmt.Errorf("claude auth: %w", err)
	}
	os.Setenv(auth.OAuthTokenEnv, token)

	client, restConfig, err := pod.NewClient(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("creating pod (image=%s, cpu=%s, mem=%s)...\n", cfg.Image, cfg.Resources.CPU, cfg.Resources.Memory)
	p, err := pod.Create(ctx, client, cfg)
	if err != nil {
		return err
	}
	podName := p.Name
	fmt.Printf("pod %s created, waiting for ready...\n", podName)

	// Ensure cleanup on any exit path
	defer func() {
		fmt.Printf("deleting pod %s...\n", podName)
		cleanupCtx := context.Background()
		if err := pod.Delete(cleanupCtx, client, cfg.Namespace, podName); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", err)
		} else {
			fmt.Printf("pod %s deleted\n", podName)
		}
	}()

	if err := pod.WaitReady(ctx, client, cfg.Namespace, podName); err != nil {
		return err
	}
	fmt.Printf("pod %s is running\n", podName)

	// Inject workspace, credentials, and env vars
	if err := pod.InjectFiles(client, restConfig, cfg, cfg.Namespace, podName); err != nil {
		return err
	}

	// Run setup script if configured
	if err := pod.RunSetup(client, restConfig, cfg, cfg.Namespace, podName); err != nil {
		return err
	}

	// Seed claude config to skip onboarding
	if err := pod.SeedClaudeConfig(client, restConfig, cfg.Namespace, podName); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not seed claude config: %v\n", err)
	}

	// Seed git user config from host
	if err := pod.SeedGitConfig(client, restConfig, cfg.Namespace, podName, cfg.Workspace); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not seed git config: %v\n", err)
	}

	// Attach to Claude Code session (blocking)
	fmt.Printf("attaching to claude code in pod %s...\n", podName)
	if cfg.SSHAuthorizedKey != "" {
		envVars := pod.BuildEnvVars(cfg.EnvVars)
		if err := pod.AttachSSH(client, restConfig, cfg.Namespace, podName, cfg, envVars); err != nil {
			fmt.Fprintf(os.Stderr, "session ended with error: %v\n", err)
		}
	} else {
		if err := pod.Attach(client, restConfig, cfg.Namespace, podName); err != nil {
			fmt.Fprintf(os.Stderr, "session ended with error: %v\n", err)
		}
	}
	fmt.Printf("session ended\n")

	if syncBack {
		if err := yolosync.GitBack(client, restConfig, cfg.Namespace, podName, cfg.Workspace); err != nil {
			fmt.Fprintf(os.Stderr, "warning: git sync-back failed: %v\n", err)
		}
	}

	return nil
}
