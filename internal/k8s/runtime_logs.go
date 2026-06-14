package k8s

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

type RuntimeLogSample struct {
	Pod       string   `json:"pod"`
	Container string   `json:"container"`
	Status    string   `json:"status"`
	Text      string   `json:"text"`
	Lines     []string `json:"lines"`
	Error     string   `json:"error,omitempty"`
}

type RuntimeLogs struct {
	Target    RuntimeMetricsTarget `json:"target"`
	Available bool                 `json:"available"`
	Error     string               `json:"error,omitempty"`
	TailLines int64                `json:"tailLines"`
	Samples   []RuntimeLogSample   `json:"samples"`
	UpdatedAt string               `json:"updatedAt"`
}

func GetRuntimeLogs(ctx context.Context, namespace string, cfg *RuntimeConfig, tailLines int64) (RuntimeLogs, error) {
	namespace = strings.TrimSpace(namespace)
	if cfg != nil && namespace == "" {
		namespace = cfg.Namespace
	}
	out := RuntimeLogs{
		Target: RuntimeMetricsTarget{
			Namespace: namespace,
		},
		TailLines: tailLines,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if cfg != nil {
		out.Target.WorkloadName = cfg.WorkloadName
		out.Target.WorkloadKind = cfg.WorkloadKind
		out.Target.Container = cfg.Container
	}
	if namespace == "" {
		return out, fmt.Errorf("namespace is required")
	}
	if tailLines <= 0 || tailLines > 1000 {
		tailLines = 200
		out.TailLines = tailLines
	}

	pods, err := podsForRuntimeConfig(ctx, namespace, cfg)
	if err != nil {
		return out, err
	}
	if len(pods) == 0 {
		out.Error = "no pods found for runtime target"
		return out, nil
	}
	out.Target.Pod = pods[0].Name

	config, err := ctrl.GetConfig()
	if err != nil {
		return out, fmt.Errorf("kubernetes config unavailable: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return out, fmt.Errorf("kubernetes client unavailable: %w", err)
	}

	for _, pod := range pods {
		for _, container := range containersForMetrics(pod, cfg) {
			sample := RuntimeLogSample{
				Pod:       pod.Name,
				Container: container.Name,
				Status:    string(pod.Status.Phase),
			}
			text, err := readContainerLogs(ctx, clientset, namespace, pod.Name, container.Name, tailLines)
			if err != nil {
				sample.Error = err.Error()
			} else {
				sample.Text = text
				sample.Lines = splitLogLines(text)
				out.Available = true
			}
			out.Samples = append(out.Samples, sample)
		}
	}
	sort.Slice(out.Samples, func(i, j int) bool {
		if out.Samples[i].Pod != out.Samples[j].Pod {
			return out.Samples[i].Pod < out.Samples[j].Pod
		}
		return out.Samples[i].Container < out.Samples[j].Container
	})
	if !out.Available && out.Error == "" {
		out.Error = "no container logs returned"
	}
	return out, nil
}

func readContainerLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, pod, container string, tailLines int64) (string, error) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container:  container,
		TailLines:  &tailLines,
		Timestamps: true,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func splitLogLines(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}
	return lines
}
