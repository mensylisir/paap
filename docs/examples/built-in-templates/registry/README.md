# Docker Registry Built-in Template

Lightweight OCI image registry based on `registry:2`.

This template is intended for local and small PAAP environments where Harbor is too heavy.
It stores image layers on a PVC and exposes an internal ClusterIP service with TLS enabled.

PAAP injects the environment runtime registry host into `tls.commonName` when this template is installed through a `ServiceInstance`.
For a production cluster, expose that same host through DNS plus Ingress or LoadBalancer and use a CA trusted by node runtimes and kpack build pods.
For kind or self-managed nodes, `deploy/k8s/paap-node-registry-agent.yaml` can be applied manually to write Docker/containerd trust config, but it is not part of the default production path.
