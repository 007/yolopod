package pod

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/007/yolopod/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClient(cfg *config.Config) (*kubernetes.Clientset, *rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if cfg.Kubecontext != "" {
		overrides.CurrentContext = cfg.Kubecontext
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	return clientset, restConfig, nil
}

func Create(ctx context.Context, client *kubernetes.Clientset, cfg *config.Config) (*corev1.Pod, error) {
	name := fmt.Sprintf("yolopod-%s", randomSuffix())

	envVars := make([]corev1.EnvVar, 0, len(cfg.EnvVars))
	for _, key := range cfg.EnvVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: "", // placeholder; inject step will set actual values
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app":        "yolopod",
				"managed-by": "yolopod",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "sandbox",
					Image: cfg.Image,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cfg.Resources.CPU),
							corev1.ResourceMemory: resource.MustParse(cfg.Resources.Memory),
						},
					},
					Env:   envVars,
					Stdin: true,
					TTY:   true,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	created, err := client.CoreV1().Pods(cfg.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating pod: %w", err)
	}

	return created, nil
}

func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
