package pod

import (
	"fmt"
	"os"

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
