package k8s

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
)

func StreamPodConsole(ctx context.Context, target RuntimeMetricsTarget, stdin io.Reader, stdout, stderr io.Writer) error {
	target.Namespace = strings.TrimSpace(target.Namespace)
	target.Pod = strings.TrimSpace(target.Pod)
	target.Container = strings.TrimSpace(target.Container)
	if target.Namespace == "" || target.Pod == "" || target.Container == "" {
		return fmt.Errorf("namespace, pod and container are required")
	}
	err := streamPodExec(ctx, target, stdin, stdout, stderr)
	if err == nil || !shouldFallbackToAttach(err) {
		return err
	}
	_, _ = io.WriteString(stdout, fmt.Sprintf("\r\nNo shell was found in container %s; opening a PAAP debug container in the same Pod.\r\n", target.Container))
	debugTarget, err := ensurePodDebugContainer(ctx, target, ConsoleDebugImage(), stdout)
	if err != nil {
		return fmt.Errorf("no shell found in target container and debug container could not be started: %w", err)
	}
	_, _ = io.WriteString(stdout, fmt.Sprintf("Connected through debug container %s. It shares the Pod network namespace with %s.\r\n", debugTarget.Container, target.Container))
	return streamPodExec(ctx, debugTarget, stdin, stdout, stderr)
}

func streamPodExec(ctx context.Context, target RuntimeMetricsTarget, stdin io.Reader, stdout, stderr io.Writer) error {
	config, err := ctrl.GetConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	var lastErr error
	for _, command := range podConsoleCommands() {
		lastErr = probePodExecCommand(ctx, config, clientset, target, command)
		if !shouldTryNextShell(lastErr) {
			if lastErr != nil {
				return lastErr
			}
			return streamPodExecCommand(ctx, config, clientset, target, command, stdin, stdout, stderr)
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no shell command candidates were configured")
}

func probePodExecCommand(ctx context.Context, config *rest.Config, clientset *kubernetes.Clientset, target RuntimeMetricsTarget, command []string) error {
	probe := podConsoleProbeCommand(command)
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(target.Pod).
		Namespace(target.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: target.Container,
			Command:   probe,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, k8sscheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Tty:    false,
	})
}

func streamPodExecCommand(ctx context.Context, config *rest.Config, clientset *kubernetes.Clientset, target RuntimeMetricsTarget, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(target.Pod).
		Namespace(target.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: target.Container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, k8sscheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    true,
	})
}

func streamPodAttach(ctx context.Context, target RuntimeMetricsTarget, stdin io.Reader, stdout, stderr io.Writer) error {
	config, err := ctrl.GetConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(target.Pod).
		Namespace(target.Namespace).
		SubResource("attach").
		VersionedParams(&corev1.PodAttachOptions{
			Container: target.Container,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, k8sscheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    true,
	})
}

func ensurePodDebugContainer(ctx context.Context, target RuntimeMetricsTarget, image string, stdout io.Writer) (RuntimeMetricsTarget, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return RuntimeMetricsTarget{}, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return RuntimeMetricsTarget{}, err
	}
	namespace := strings.TrimSpace(target.Namespace)
	podName := strings.TrimSpace(target.Pod)
	targetContainer := strings.TrimSpace(target.Container)
	if namespace == "" || podName == "" || targetContainer == "" {
		return RuntimeMetricsTarget{}, fmt.Errorf("namespace, pod and target container are required")
	}

	for attempt := 0; attempt < 3; attempt++ {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return RuntimeMetricsTarget{}, err
		}
		if name, ok := reusableDebugContainerName(pod, targetContainer); ok {
			next := target
			next.Container = name
			return next, nil
		}
		name := availableDebugContainerName(pod, targetContainer)
		nextPod := pod.DeepCopy()
		nextPod.Spec.EphemeralContainers = append(nextPod.Spec.EphemeralContainers, debugEphemeralContainer(name, image, targetContainer))
		if stdout != nil {
			_, _ = io.WriteString(stdout, fmt.Sprintf("Starting debug container %s with image %s...\r\n", name, image))
		}
		if _, err := clientset.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, nextPod, metav1.UpdateOptions{}); err != nil {
			if apierrors.IsConflict(err) {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			return RuntimeMetricsTarget{}, err
		}
		if err := waitForDebugContainer(ctx, clientset, namespace, podName, name); err != nil {
			return RuntimeMetricsTarget{}, err
		}
		next := target
		next.Container = name
		return next, nil
	}
	return RuntimeMetricsTarget{}, fmt.Errorf("pod %s/%s changed while adding debug container", namespace, podName)
}

func debugEphemeralContainer(name, image, targetContainer string) corev1.EphemeralContainer {
	return corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            name,
			Image:           strings.TrimSpace(image),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"/bin/sh",
				"-c",
				"trap 'exit 0' TERM INT; while true; do sleep 3600; done",
			},
			Stdin:                    true,
			TTY:                      true,
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		},
		TargetContainerName: targetContainer,
	}
}

func waitForDebugContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string) error {
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("debug container %s did not become ready before timeout", containerName)
		case <-ticker.C:
			pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			status, ok := debugContainerStatus(pod, containerName)
			if !ok {
				continue
			}
			if status.State.Running != nil {
				return nil
			}
			if status.State.Terminated != nil {
				terminated := status.State.Terminated
				return fmt.Errorf("debug container %s terminated: %s %s", containerName, terminated.Reason, terminated.Message)
			}
			if status.State.Waiting != nil {
				waiting := status.State.Waiting
				switch waiting.Reason {
				case "ErrImagePull", "ImagePullBackOff", "InvalidImageName", "CreateContainerConfigError", "CreateContainerError":
					return fmt.Errorf("debug container %s is waiting: %s %s", containerName, waiting.Reason, waiting.Message)
				}
			}
		}
	}
}

func reusableDebugContainerName(pod *corev1.Pod, targetContainer string) (string, bool) {
	prefix := debugContainerNamePrefix(targetContainer)
	for _, container := range pod.Spec.EphemeralContainers {
		if container.TargetContainerName != targetContainer || !strings.HasPrefix(container.Name, prefix) {
			continue
		}
		status, ok := debugContainerStatus(pod, container.Name)
		if !ok || status.State.Running != nil {
			return container.Name, true
		}
	}
	return "", false
}

func availableDebugContainerName(pod *corev1.Pod, targetContainer string) string {
	used := map[string]bool{}
	for _, container := range pod.Spec.EphemeralContainers {
		used[container.Name] = true
	}
	prefix := debugContainerNamePrefix(targetContainer)
	if !used[prefix] {
		return prefix
	}
	for i := 2; i < 100; i++ {
		name := fmt.Sprintf("%s-%d", trimDebugContainerPrefix(prefix, 63-len(fmt.Sprintf("-%d", i))), i)
		if !used[name] {
			return name
		}
	}
	return fmt.Sprintf("%s-%d", trimDebugContainerPrefix(prefix, 52), time.Now().Unix())
}

func debugContainerNamePrefix(targetContainer string) string {
	name := "paap-debug-" + dns1123Name(targetContainer)
	return trimDebugContainerPrefix(name, 52)
}

func trimDebugContainerPrefix(value string, max int) string {
	value = strings.Trim(value, "-")
	if max <= 0 {
		return "paap-debug"
	}
	if len(value) <= max {
		return value
	}
	return strings.Trim(value[:max], "-")
}

var dns1123Cleaner = regexp.MustCompile(`[^a-z0-9-]+`)

func dns1123Name(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	name = dns1123Cleaner.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "container"
	}
	return name
}

func debugContainerStatus(pod *corev1.Pod, containerName string) (corev1.ContainerStatus, bool) {
	for _, status := range pod.Status.EphemeralContainerStatuses {
		if status.Name == containerName {
			return status, true
		}
	}
	return corev1.ContainerStatus{}, false
}

func ConsoleDebugImage() string {
	if image := strings.TrimSpace(os.Getenv("PAAP_CONSOLE_DEBUG_IMAGE")); image != "" {
		return image
	}
	return "busybox:1.36"
}

func podConsoleCommands() [][]string {
	return [][]string{
		{"/bin/bash", "-l"},
		{"/usr/bin/bash", "-l"},
		{"/usr/local/bin/bash", "-l"},
		{"/bin/sh"},
		{"/usr/bin/sh"},
		{"/usr/local/bin/sh"},
		{"/busybox/sh"},
	}
}

func podConsoleProbeCommand(command []string) []string {
	if len(command) == 0 {
		return []string{"/bin/sh", "-c", "exit 0"}
	}
	shell := command[0]
	return []string{shell, "-c", "exit 0"}
}

func shouldTryNextShell(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"no such file",
		"not found",
		"executable file not found",
		"stat /bin/",
		"stat /usr/",
		"stat /busybox/",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func shouldFallbackToAttach(err error) bool {
	return shouldTryNextShell(err)
}

func podConsoleCommand() []string {
	return []string{
		"/bin/sh",
		"-lc",
		strings.Join([]string{
			`if command -v bash >/dev/null 2>&1; then exec bash; fi`,
			`if command -v sh >/dev/null 2>&1; then exec sh; fi`,
			`echo "No interactive shell was found in this container."`,
			`sleep 2`,
		}, "\n"),
	}
}
