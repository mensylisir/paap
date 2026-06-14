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
