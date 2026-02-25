package pod

import (
	"fmt"
	"os"

	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func Attach(client *kubernetes.Clientset, restConfig *rest.Config, namespace, podName string) error {
	// Put terminal into raw mode for proper TTY passthrough
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("setting raw terminal: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "sandbox",
		Command:   []string{"claude", "--dangerously-skip-permissions"},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("creating executor: %w", err)
	}

	termSize := remotecommand.TerminalSize{}
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err == nil {
		termSize.Width = uint16(w)
		termSize.Height = uint16(h)
	}

	sizeQueue := &fixedSizeQueue{size: termSize}

	return exec.Stream(remotecommand.StreamOptions{
			Stdin:             os.Stdin,
			Stdout:            os.Stdout,
			Stderr:            os.Stderr,
			Tty:               true,
			TerminalSizeQueue: sizeQueue,
	})
}

type fixedSizeQueue struct {
	size    remotecommand.TerminalSize
	started bool
}

func (q *fixedSizeQueue) Next() *remotecommand.TerminalSize {
	if !q.started {
		q.started = true
		return &q.size
	}
	// Block forever after initial size; a proper implementation would
	// listen for SIGWINCH, but this is sufficient for MVP.
	select {}
}
