package sync

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/007/yolopod/internal/pod"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GitBack(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName, localWorkspace string) error {
	// Check if there are any changes in the pod
	hasChanges, err := checkChanges(client, restConfig, namespace, podName)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}

	if !hasChanges {
		fmt.Println("no git changes detected in pod")
		return nil
	}

	fmt.Println("git changes detected, syncing back...")

	// Tar the .git directory from the pod
	gitTar, err := tarFromPod(client, restConfig, namespace, podName)
	if err != nil {
		return fmt.Errorf("extracting .git from pod: %w", err)
	}

	// Extract to local workspace
	absWorkspace, err := filepath.Abs(localWorkspace)
	if err != nil {
		return fmt.Errorf("resolving workspace: %w", err)
	}

	if err := extractTar(gitTar, absWorkspace); err != nil {
		return fmt.Errorf("extracting to local workspace: %w", err)
	}

	// Show what changed
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = absWorkspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	return nil
}

func checkChanges(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string) (bool, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Check for uncommitted changes or new commits
	err := pod.ExecWithConfig(client, restConfig, pod.ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   []string{"bash", "-c", "cd /workspace && git status --porcelain && git log --oneline @{push}.. 2>/dev/null || true"},
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(stdout.String()) != "", nil
}

func tarFromPod(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string) (*bytes.Buffer, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Tar up the entire workspace (includes .git and working tree)
	err := pod.ExecWithConfig(client, restConfig, pod.ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   []string{"tar", "cf", "-", "-C", "/workspace", "."},
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("tar from pod: %w (stderr: %s)", err, stderr.String())
	}

	return &stdout, nil
}

func extractTar(data *bytes.Buffer, destDir string) error {
	tr := tar.NewReader(data)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, hdr.Name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) && filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("invalid tar path: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}
