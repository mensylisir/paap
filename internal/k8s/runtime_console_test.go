package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestConsoleDebugImageDefaultsToPreloadedBusybox(t *testing.T) {
	t.Setenv("PAAP_CONSOLE_DEBUG_IMAGE", "")

	if got := ConsoleDebugImage(); got != "busybox:1.36" {
		t.Fatalf("default debug image = %q, want busybox:1.36", got)
	}

	t.Setenv("PAAP_CONSOLE_DEBUG_IMAGE", "docker.io/nicolaka/netshoot:v0.13")
	if got := ConsoleDebugImage(); got != "docker.io/nicolaka/netshoot:v0.13" {
		t.Fatalf("configured debug image = %q", got)
	}
}

func TestDebugContainerNameUsesReusableRunningContainer(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{EphemeralContainers: []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "paap-debug-frontend-1"},
				TargetContainerName:      "frontend-1",
			},
		}},
		Status: corev1.PodStatus{EphemeralContainerStatuses: []corev1.ContainerStatus{
			{
				Name:  "paap-debug-frontend-1",
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			},
		}},
	}

	name, ok := reusableDebugContainerName(pod, "frontend-1")
	if !ok || name != "paap-debug-frontend-1" {
		t.Fatalf("reusable debug container = %q/%v", name, ok)
	}
}

func TestAvailableDebugContainerNameSkipsTerminatedContainers(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{EphemeralContainers: []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "paap-debug-frontend-1"},
				TargetContainerName:      "frontend-1",
			},
		}},
		Status: corev1.PodStatus{EphemeralContainerStatuses: []corev1.ContainerStatus{
			{
				Name:  "paap-debug-frontend-1",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}},
			},
		}},
	}

	if name, ok := reusableDebugContainerName(pod, "frontend-1"); ok {
		t.Fatalf("terminated debug container should not be reused, got %q", name)
	}
	if got := availableDebugContainerName(pod, "frontend-1"); got != "paap-debug-frontend-1-2" {
		t.Fatalf("available debug container name = %q", got)
	}
}

func TestDebugEphemeralContainerDoesNotSetForbiddenResources(t *testing.T) {
	container := debugEphemeralContainer("paap-debug-frontend-1", "busybox:1.36", "frontend-1")

	if len(container.Resources.Requests) != 0 || len(container.Resources.Limits) != 0 {
		t.Fatalf("debug ephemeral container must not set resources, got %#v", container.Resources)
	}
	if len(container.Command) != 1 || container.Command[0] != "/bin/sh" {
		t.Fatalf("debug ephemeral container must run an attachable shell, got %#v", container.Command)
	}
	if container.TargetContainerName != "frontend-1" {
		t.Fatalf("target container = %q", container.TargetContainerName)
	}
}

func TestDebugContainerNameSkipsLegacySleepContainers(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{EphemeralContainers: []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Name:    "paap-debug-frontend-1",
					Command: []string{"/bin/sh", "-c", "while true; do sleep 3600; done"},
				},
				TargetContainerName: "frontend-1",
			},
		}},
		Status: corev1.PodStatus{EphemeralContainerStatuses: []corev1.ContainerStatus{
			{
				Name:  "paap-debug-frontend-1",
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			},
		}},
	}

	if name, ok := reusableDebugContainerName(pod, "frontend-1"); ok {
		t.Fatalf("legacy sleep debug container should not be reused for attach, got %q", name)
	}
	if got := availableDebugContainerName(pod, "frontend-1"); got != "paap-debug-frontend-1-2" {
		t.Fatalf("available debug container name = %q", got)
	}
}

func TestPodConsoleProbeCommandDoesNotUseInteractiveFlags(t *testing.T) {
	got := podConsoleProbeCommand([]string{"/bin/bash", "-l"})
	want := []string{"/bin/bash", "-c", "exit 0"}
	if len(got) != len(want) {
		t.Fatalf("probe command length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("probe command[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if shouldTryNextShell(nil) {
		t.Fatal("nil error should not try next shell")
	}
}
