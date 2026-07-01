# PAAP Agent Guide

This file is the stable project guide for agents working on PAAP. It should contain durable project facts, architecture boundaries, development rules, and current design direction. Task lists and implementation plans belong in `docs/tasks.md`.

## Product Context

PAAP is a Railway-like self-service cloud-native application platform. Users create applications and environments, install platform services such as Gitea, Harbor, Argo CD, Jenkins, PostgreSQL, Redis, Kafka, RabbitMQ, MinIO, Prometheus/Grafana, and Loki, then deploy business components through a canvas UI and right drawers.

The product should hide Kubernetes jargon from ordinary users. Default service and component views should prioritize:

- endpoint / address
- port
- username
- password or token, with show/hide controls
- connection string
- usage examples
- monitoring and log entry points

Kubernetes details such as namespace, StatefulSet, Pod, PVC, Helm release, CRD, and ServiceAccount belong in an advanced or operations section, not the main business view.

## Current Architecture

PAAP currently uses a three-process single-cluster architecture:

```text
Vue 3 frontend
  -> PAAP Server (Go + Gin + GORM + PostgreSQL)
  -> Kubernetes CRDs
  -> PAAP Operator (controller-runtime)
  -> Kubernetes workloads and Helm releases
```

Main code areas:

- `cmd/server/main.go`: server entrypoint, database init, migrations/seeding, cluster sync, HTTP server.
- `internal/handler/`: REST handlers. Most application, environment, service, capability, auth, runtime, proxy, and workspace endpoints live here.
- `internal/model/`: GORM models.
- `internal/service/`: business helpers such as template rendering, cluster sync, tool workspace construction, and component GitOps.
- `internal/k8s/`: Kubernetes client helpers and product-specific clients.
- `cmd/operator/main.go` and `internal/controller/`: Kubernetes controllers for Application, Environment, ServiceInstance, and Component CRDs.
- `api/v1/`: CRD Go types.
- `frontend/src/views/EnvDetailView.vue`: main environment canvas and drawers. It is large; prefer extracting focused composables/components when adding substantial behavior.
- `frontend/src/components/workspaces/`: product-specific service workspace components.
- `frontend/src/api/client.ts`: frontend API client.
- `data/charts/`: built-in Helm chart packages currently embedded into the server image.
- `deploy/k8s/`: local kind deployment manifests and scripts.

## Current Domain Model

Important existing models:

- `Application`: a PAAP application. System applications use `IsSystem`.
- `Environment`: an application environment. Current model has only `Namespace`; there is no `ClusterID` yet.
- `ServiceInstallation`: a PAAP-managed service installed in an environment. Current uniqueness is `environment_id + service_type`, so one environment cannot hold two instances of the same service type without schema changes.
- `EnvironmentCapability`: a capability reference for an environment. Current sources include `managed`, `shared`, `external`, and `deferred`.
- `Component`: a business component deployed in an environment.
- `ComponentConfig.Bindings`: JSON binding data between components and services, useful but not strong enough as the only platform usage source.
- `User`, `UserRole`, and `AppMember`: authentication, primary user role, and application membership.

Do not treat canvas edges as authoritative business relationships. They are UI layout/visual relationships. Platform-level reporting needs a real service usage relation model or a reliable read model built from structured sources.

## Capability Direction

Tools and middleware must support multiple consumption and delivery modes:

- `managed`: PAAP installs and owns the service in an environment.
- `shared`: a business environment references a PAAP-owned shared service from the system shared resource pool.
- `external`: PAAP stores a connection record and credential reference for a service outside PAAP ownership.
- `kubevirt`: PAAP creates a service instance from a KubeVirt-backed service template.

Shared services are PAAP-owned and may be installed through the shared resource pool. External services are not PAAP-owned. Deleting or disconnecting an external capability must only remove PAAP's connection record and generated local credentials; it must never delete the real external system.

KubeVirt is platform infrastructure, not a user-facing "create an empty VM" feature. Users should create service products such as PostgreSQL, Redis, or MySQL; PAAP may satisfy that service instance through a KubeVirt service template that creates a `VirtualMachine`, Kubernetes `Service`, credentials, readiness checks, monitoring targets, and standard connection outputs.

External credentials should be stored in Kubernetes Secrets or another credential backend and referenced from the database. Do not hard-code platform credentials in application code. Schema/data changes should be expressed through migrations where practical.

## Platform Service Direction

Leadership's current product direction is to evolve PAAP from an environment-level middleware installer into a platform service catalog and service instance operations system.

Use three conceptual layers:

- Service product: what the platform offers, such as database, cache, message queue, Git, registry, CI, CD, monitoring, logging, storage, network, DNS, or Ingress.
- Service instance: a concrete managed, shared, external, or KubeVirt-template-backed instance.
- Service usage relation: which application, environment, or component uses which instance, through which feature, source, and provision mode.

The current architecture can support this direction, but it needs a service domain/read-model layer. Do not continue adding unrelated per-page aggregation logic for platform service usage.

## Multi-Cluster Direction

The current implementation is single-cluster:

- There is no `Cluster` model.
- `Environment` has `Namespace`, not `ClusterID`.
- `ServiceInstallation` stores namespace but not cluster.
- many runtime, credential, monitor, and log paths use the global Kubernetes client.
- the operator reconciles the current cluster only.

If PAAP later manages other clusters or deploys applications to other clusters, this requires an architecture extension, not just UI changes. The preferred future architecture is:

```text
PAAP central control plane
  -> cluster registry and scheduling
  -> desired state per target cluster
  -> per-cluster PAAP operator/agent
  -> status, metrics, logs, and service inventory reported back
```

Future multi-cluster work should add at least:

- `Cluster`
- `Environment.ClusterID`
- `ServiceInstallation.ClusterID`
- `EnvironmentCapability.ClusterID` or inheritance from environment
- `ServiceUsageRelation.ClusterID`
- a cluster-aware Kubernetes client/provider instead of global `k8s.GetClient()` usage in new platform-level logic

Shared resource pools are normally cluster-scoped. A shared Redis in cluster A should not be assumed reachable from cluster B unless it is modeled as an external service or cross-cluster networking has been explicitly implemented.

## Development Commands

Go requires GVM Go 1.25+:

```bash
source ~/.gvm/scripts/gvm && gvm use go1.25.7
```

Common commands:

```bash
make run
make run-operator
make build
make build-operator
make test
make frontend-test
make frontend-verify
make verify
make fmt
make lint
make docker-build-server
make docker-build-operator
```

For focused tests, prefer targeted commands first, then broader verification:

```bash
source ~/.gvm/scripts/gvm && gvm use go1.25.7 && go test ./internal/handler -run TestName -count=1 -v
npm --prefix frontend run test -- src/views/viewMarkup.test.ts --run
npm --prefix frontend run build
```

## Deployment And Runtime Rules

Local cluster context:

```bash
kubectl --context kind-rbac-governance-test ...
```

The local kind cluster cannot reliably pull new images. Build locally and load images before applying manifests that reference them:

```bash
kind load docker-image --name rbac-governance-test paap-server:<tag>
```

When changing deployment image tags, keep `Makefile` and deployment manifests/scripts consistent. Check the real current tag before deploying so a stale image does not overwrite a newer one.

For browser/UI verification, use visible Chrome via CDP when validating user-facing behavior. Do not rely only on headless smoke tests for UI changes that affect canvas, drawers, menus, forms, or runtime data.

## Documentation Split

- `AGENT.md`: durable project facts, architecture, constraints, operating rules.
- `docs/tasks.md`: current tasks, branch plan, architecture proposals, acceptance criteria, historical task log, and broader backlog.
- `CLAUDE.md`: legacy Claude-specific context. Valuable project facts should be mirrored here or in `docs/tasks.md` instead of expanding that file further.
