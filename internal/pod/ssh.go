package pod

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/007/yolopod/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func prepareSSH(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName, pubKey string, envVars []corev1.EnvVar) error {
	// Generate host keys, set up authorized_keys, configure and start sshd
	setupScript := fmt.Sprintf(`
sudo ssh-keygen -A &&
mkdir -p /home/coder/.ssh &&
chmod 700 /home/coder/.ssh &&
echo %q > /home/coder/.ssh/authorized_keys &&
chmod 600 /home/coder/.ssh/authorized_keys &&
sudo tee /etc/ssh/sshd_config.d/yolopod.conf > /dev/null <<'SSHD'
AllowAgentForwarding yes
PermitUserEnvironment yes
SSHD
sudo /usr/sbin/sshd
`, pubKey)

	if err := execSimple(client, restConfig, namespace, podName, []string{"bash", "-c", setupScript}); err != nil {
		return fmt.Errorf("sshd setup: %w", err)
	}

	// Write env vars to SSH environment file so the session inherits them.
	// PATH is handled via ". ~/.profile" in the SSH command since PAM
	// overrides PATH from .ssh/environment with /etc/environment.
	if len(envVars) > 0 {
		var lines []string
		for _, ev := range envVars {
			lines = append(lines, fmt.Sprintf("%s=%s", ev.Name, ev.Value))
		}
		envContent := strings.Join(lines, "\n")
		envScript := fmt.Sprintf("cat > /home/coder/.ssh/environment << 'EOF'\n%s\nEOF", envContent)
		if err := execSimple(client, restConfig, namespace, podName, []string{"bash", "-c", envScript}); err != nil {
			return fmt.Errorf("writing ssh environment: %w", err)
		}
	}

	// Copy host ~/.ssh/config into the pod so host aliases work
	home, err := os.UserHomeDir()
	if err == nil {
		sshConfig := filepath.Join(home, ".ssh", "config")
		if _, statErr := os.Stat(sshConfig); statErr == nil {
			fmt.Println("injecting ssh config...")
			if copyErr := copySingleFile(client, restConfig, namespace, podName, sshConfig, "/home/coder/.ssh/config"); copyErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not inject ssh config: %v\n", copyErr)
			}
		}
	}

	return nil
}

func startPortForward(restConfig *rest.Config, namespace, podName string) (int, chan struct{}, error) {
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return 0, nil, fmt.Errorf("creating round tripper: %w", err)
	}

	pfURL, err := url.Parse(fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward", restConfig.Host, namespace, podName))
	if err != nil {
		return 0, nil, fmt.Errorf("parsing port-forward URL: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", pfURL)

	stopChan := make(chan struct{})
	readyChan := make(chan struct{})

	// Port 0 means random local port
	fw, err := portforward.New(dialer, []string{"0:22"}, stopChan, readyChan, nil, os.Stderr)
	if err != nil {
		return 0, nil, fmt.Errorf("creating port forwarder: %w", err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- fw.ForwardPorts()
	}()

	select {
	case <-readyChan:
	case err := <-errChan:
		return 0, nil, fmt.Errorf("port forward failed: %w", err)
	}

	ports, err := fw.GetPorts()
	if err != nil {
		close(stopChan)
		return 0, nil, fmt.Errorf("getting forwarded ports: %w", err)
	}

	return int(ports[0].Local), stopChan, nil
}

func AttachSSH(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string, cfg *config.Config, envVars []corev1.EnvVar) error {
	if err := prepareSSH(client, restConfig, namespace, podName, cfg.SSHAuthorizedKey, envVars); err != nil {
		return err
	}

	localPort, stopChan, err := startPortForward(restConfig, namespace, podName)
	if err != nil {
		return err
	}
	defer close(stopChan)

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh binary not found: %w", err)
	}

	cmd := exec.Command(sshBin,
		"-A", "-t",
		"-p", strconv.Itoa(localPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"coder@127.0.0.1",
		". ~/.profile && cd /workspace && exec claude --dangerously-skip-permissions",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
