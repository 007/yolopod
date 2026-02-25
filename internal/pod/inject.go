package pod

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/007/yolopod/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func SeedClaudeConfig(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Create the .claude dir in the container
	if err := execSimple(client, restConfig, namespace, podName, []string{"mkdir", "-p", "/home/coder/.claude"}); err != nil {
		return err
	}

	// Write ~/.claude.json to skip onboarding
	script := `cat > /home/coder/.claude.json << 'EOF'
{"hasCompletedOnboarding":true,"autoUpdates":false}
EOF`
	if err := execSimple(client, restConfig, namespace, podName, []string{"bash", "-c", script}); err != nil {
		return fmt.Errorf("writing .claude.json: %w", err)
	}

	// Copy host settings.json if it exists
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		fmt.Println("injecting claude settings...")
		if err := copySingleFile(client, restConfig, namespace, podName, settingsPath, "/home/coder/.claude/settings.json"); err != nil {
			return fmt.Errorf("injecting settings.json: %w", err)
		}
	}

	return nil
}

func InjectFiles(client *kubernetes.Clientset, restConfig *rest.Config, cfg *config.Config, namespace, podName string) error {
	// Inject workspace
	workspace, err := filepath.Abs(cfg.Workspace)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}

	fmt.Printf("injecting workspace %s...\n", workspace)
	if err := copyToPod(client, restConfig, namespace, podName, workspace, "/workspace"); err != nil {
		return fmt.Errorf("injecting workspace: %w", err)
	}

	// Inject credentials
	for _, cred := range cfg.Credentials {
		local := expandHome(cred.Local)
		if _, err := os.Stat(local); os.IsNotExist(err) {
			fmt.Printf("warning: credential file %s not found, skipping\n", local)
			continue
		}
		fmt.Printf("injecting credential %s -> %s...\n", cred.Local, cred.Remote)

		dir := filepath.Dir(cred.Remote)
		if err := execSimple(client, restConfig, namespace, podName, []string{"mkdir", "-p", dir}); err != nil {
			return fmt.Errorf("creating credential dir %s: %w", dir, err)
		}

		if err := copySingleFile(client, restConfig, namespace, podName, local, cred.Remote); err != nil {
			return fmt.Errorf("injecting credential %s: %w", cred.Local, err)
		}
	}

	return nil
}

func copyToPod(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName, localPath, remotePath string) error {
	var buf bytes.Buffer
	if err := createTar(&buf, localPath); err != nil {
		return fmt.Errorf("creating tar: %w", err)
	}

	return ExecWithConfig(client, restConfig, ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   []string{"tar", "xf", "-", "-C", remotePath},
		Stdin:     &buf,
		Stderr:    os.Stderr,
	})
}

func copySingleFile(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name: filepath.Base(remotePath),
		Mode: 0600,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	tw.Close()

	dir := filepath.Dir(remotePath)
	return ExecWithConfig(client, restConfig, ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   []string{"tar", "xf", "-", "-C", dir},
		Stdin:     &buf,
		Stderr:    os.Stderr,
	})
}

func createTar(w io.Writer, srcDir string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to source dir
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory entry itself
		if relPath == "." {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			hdr.Linkname = link
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func execSimple(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string, command []string) error {
	return ExecWithConfig(client, restConfig, ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: "sandbox",
		Command:   command,
		Stderr:    os.Stderr,
	})
}

func SeedGitConfig(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName, workspace string) error {
	name := localGitConfig(workspace, "user.name")
	email := localGitConfig(workspace, "user.email")
	if name == "" {
		name = "yolopod"
	}
	if email == "" {
		email = "yolopod@localhost"
	}

	script := fmt.Sprintf(`git config --global user.name %q && git config --global user.email %q`, name, email)
	return execSimple(client, restConfig, namespace, podName, []string{"bash", "-c", script})
}

func localGitConfig(dir, key string) string {
	cmd := exec.Command("git", "config", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
