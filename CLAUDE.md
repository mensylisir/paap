# PAAP Agent Notes

## Unfinished Work And Known Gaps

Do not treat the long-running Railway-like drawer objective as complete until every item below has direct code, runtime, and CDP evidence.

1. Product-specific drawers still need a full audit for every concrete tool and middleware:
   Gitea, Registry, Harbor, Argo CD, Jenkins, Prometheus/Grafana, Loki, PostgreSQL, MySQL, MongoDB, Redis, RabbitMQ, Kafka, and MinIO.
   Existing drawers are partially product-specific, but not every product has been CDP-tested end to end.

2. MongoDB, Kafka, and MinIO now use embedded drawer action forms in source, but the current local environment does not have running MongoDB/Kafka/MinIO cards for CDP verification.
   Install each one from the canvas and prove create/read/update/delete style actions call real backend APIs and refresh real workspace data.

3. RabbitMQ embedded action forms are implemented in source, but the current local environment has no RabbitMQ card.
   Install RabbitMQ from the canvas and verify queue, exchange, binding, publish, read, purge, and delete flows from the drawer.

4. Database management is not fully proven.
   PostgreSQL drawer exposes database/table/row operations and backup creation, but table create/insert/update/delete and backup output need a fresh CDP run against a real database with visible before/after evidence.
   MySQL needs the same verification, including replication/Galera modes where applicable.

5. Database backup is only partially covered.
   Backup creation is implemented, but restore/download/list details and failure-state UX still need product-level decisions and CDP proof.

6. Persistent volume configuration needs full chart-by-chart proof.
   The UI shows PV size presets for many services, but each Helm values mapping and running-instance update must be verified against actual ServiceInstance specs, Helm output, PVCs, and chart behavior.
   Kubernetes PVC expansion limitations must be surfaced in user-facing language where a live resize cannot actually happen.

7. Topology modes need end-to-end verification.
   Redis standalone, replication, Sentinel, and cluster modes are represented in config.
   PostgreSQL/MySQL standalone, replica, dual-master/Galera/HA modes are represented in config.
   Each mode still needs a canvas deploy test proving the chosen values reach Helm and result in the expected pods/services/PVCs.

8. Runtime config updates for already-running services need more proof.
   Updating ServiceInstance values is implemented for running services, but every high-risk setting needs verification that the operator/Helm path reconciles the live release without stale UI state.

9. Per-card metrics need a Railway-like visual audit.
   CPU/memory charts exist in drawers, but every component/tool/middleware card must be checked for real data, empty states, time ranges, chart scaling, and no misleading placeholder values.

10. Per-card logs need a no-placeholder audit.
    Logs are available in drawers, but every component/tool/middleware card must be checked for real log lines and no "no such host" style failures.

11. Console needs broader verification.
    Attach/debug-container fallback was fixed and verified for selected component/service cases.
    It still needs CDP checks for all common tool and middleware pods, especially images without a shell and pods where ephemeral containers are restricted.

12. Config template coverage is incomplete.
    Built-in templates exist and the component drawer has a single template dropdown, but common framework templates still need broader coverage:
    nginx multi-backend routing, Spring Boot datasource/cache/mq profiles, Gin/Go config, Node/Vite frontend API config, and config-file based apps.

13. User-provided config template upload/edit/preview needs more UX proof.
    Template management exists, but the flow must clearly show raw template content, extracted fields, sensitive fields, generated files, and validation errors without requiring users to know Kubernetes object names.

14. Automatic relationship detection is incomplete.
    Env vars and selected service references can draw relationships, but configmaps/secrets/file-based configs need deeper parsing and safe heuristics so backend-to-db/cache/mq lines appear without manual wiring.

15. Kubernetes jargon is still visible in some places.
    Review all drawers and workspaces for labels such as namespace, service, pod, configmap, secret, pvc, helm, and replace or hide them unless the user explicitly opens an advanced/debug view.

16. Registry and image-source flow needs a final real demo pass.
    The component drawer separates environment registry host from image:tag, but the normal path still needs CDP proof:
    push image to registry, create component, push manifests to repo, Argo CD deploys, pod runs from the expected image.

17. The demo environment is incomplete for full objective verification.
    Current verified environment has frontend, backend, PostgreSQL, Redis, Gitea, Argo CD, monitor, logs, and registry.
    It lacks running RabbitMQ, Kafka, MongoDB, MinIO, MySQL, Harbor, and Jenkins cards for full drawer coverage.

18. No fake or placeholder data is allowed.
    Every workspace resource, metric, log, backup, key, queue, topic, bucket, and deployment row must be traced to a real backend/API/cluster source.
    Add tests or remove UI blocks where data is synthetic.

19. CDP test coverage is still incomplete.
    Continue using the visible Chrome via CDP, not headless-only runs.
    Test every page/tab/drawer/action after each meaningful UI change.

20. Kind image loading remains required.
    The local kind cluster cannot reliably pull images.
    Always build/pull images locally and run `kind load docker-image --name rbac-manager-test ...` before applying manifests that reference new images.

21. Disk usage must be checked before and after image-heavy work.
    Current recent checks were safe, but frequent Docker builds can fill disk quickly.

## Last Known Runtime State

- Kind cluster: `rbac-manager-test`
- Last deployed PAAP server image for this note: `paap-server:v0.1.378`
- Base pushed commit before this note: `4cde3f4 Embed service workspace action forms`
- Recent verified CDP flow: Redis drawer writes a real key through the embedded form and lists it back from Redis.
