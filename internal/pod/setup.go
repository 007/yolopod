package pod

import (
	"fmt"
	"os"
	"strings"

	"github.com/007/yolopod/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func RunSetup(client *kubernetes.Clientset, restConfig *rest.Config, cfg *config.Config, namespace, podName string) error {
	if cfg.SetupScript == "" {
		return nil
	}

	script, err := os.ReadFile(cfg.SetupScript)
	if err != nil {
		return fmt.Errorf("reading setup script %s: %w", cfg.SetupScript, err)
	}

	fmt.Printf("running setup script %s...\n", cfg.SetupScript)
	return ExecWithConfig(client, restConfig, ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   []string{"bash", "-c", string(script)},
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	})
}

func InstallPackages(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	// Package names are passed unsanitized to apt-get. The config is user-controlled
	// and runs inside an ephemeral container, so there's no privilege boundary to protect.
	fmt.Printf("installing packages: %s...\n", strings.Join(packages, ", "))
	cmd := "export DEBIAN_FRONTEND=noninteractive DEBCONF_NONINTERACTIVE_SEEN=true && " +
		"sudo -E apt-get update -qq && " +
		"sudo -E apt-get install -y -qq --no-install-recommends " + strings.Join(packages, " ")
	return ExecWithConfig(client, restConfig, ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   []string{"bash", "-c", cmd},
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	})
}
