# Kind 离线镜像、模板与 CI 方案

## 模板发布链路

PAAP 的内置模板有源码态和运行态两层：

- 源码态：`docs/examples/built-in-templates/<name>/`
  - 包含 `platform-manifest.yaml`、`preset-values.yaml`、`chart/`。
  - 开发、评审、修改模板时改这里。
- 发布态：`data/charts/<name>.tar.gz`
  - 由 `scripts/package-built-in-templates.sh` 从源码态打包生成。
  - `Dockerfile.server` 会把 `data/charts/` 复制到 server 镜像的 `/charts/`。
- 运行态：MinIO
  - `deploy/k8s/init-templates.yaml` 从 `paap-server` 镜像的 `/charts/*.tar.gz` 拷贝模板包。
  - init job 使用 `minio/mc` 上传到 `paap-charts/charts/*.tar.gz`。
  - `ServiceTemplate` 记录 `S3Bucket=paap-charts` 和 `S3Key=charts/<name>.tar.gz`。
  - 安装工具时后端从 MinIO 下载 chart 包，再用 Helm 安装。

因此，`docs/examples` 不是运行时读取路径；运行时以 `data/charts -> /charts -> MinIO -> Helm install` 为准。

## Kind 集群离线镜像策略

当前 kind 集群不能从外网拉镜像，所以任何会出现在 Pod spec 里的镜像都必须先在宿主机 Docker 中存在，再导入 kind：

```bash
docker pull <image>
kind load docker-image <image> --name kind
```

项目提供统一脚本：

```bash
make preload-kind-images
```

脚本读取 `deploy/k8s/kind-images.txt`，并尝试从 `data/charts/*.tar.gz` 解包渲染 Helm chart，补充发现模板里的镜像。`deploy/k8s/deploy.sh` 已调整为先打包模板、构建 PAAP 镜像、预加载镜像，再创建任何会启动 Pod 的 Kubernetes 资源，避免 ImagePullBackOff。

模板中所有实际运行镜像应固定版本并使用 `IfNotPresent`。离线 kind 中如果镜像 tag 是 `latest` 或 `imagePullPolicy: Always`，即使镜像已经 `kind load` 进节点，也可能触发远端拉取检查并失败。

如果宿主机已经预先准备好全部镜像，可使用：

```bash
PULL_IMAGES=false make preload-kind-images
```

可选的 `deploy/k8s/paap-node-registry-agent.yaml` 也使用清单中的 `docker.io/alpine/k8s:1.34.1`。如果在 kind 中启用该 DaemonSet，也必须先把这个镜像导入 kind。

## Registry 地址与 TLS 信任模型

组件最终部署时，镜像引用由 PAAP 生成，例如：

```text
<registry-host>/<app>-<env>/<component>:<tag>
```

`<registry-host>` 来自 `PAAP_REGISTRY_HOST_TEMPLATE`，默认值是：

```text
registry.{app}-{env}.paap.local
```

支持的占位符包括 `{app}`、`{env}`、`{primaryNamespace}`、`{toolNamespace}`、`{service}`。生产集群必须把它改成节点运行时可解析、可访问、证书可信任的真实地址，例如：

```text
registry.{app}-{env}.corp.example.com:5443
harbor-{app}-{env}.corp.example.com
```

不要把业务镜像地址写成 `*.svc.cluster.local`。ArgoCD 更新的是 Deployment 镜像字段，真正拉镜像的是每个节点上的 kubelet/containerd/Docker；节点运行时通常不解析集群内 Service DNS，也不会自动信任 Service 内自签证书。

生产默认方案：

- 使用企业 DNS、Ingress 或 LoadBalancer 暴露每个环境的 registry/Harbor。
- registry/Harbor 使用 HTTPS。PAAP 的目标使用场景以企业内网为主，所以默认按企业私有 CA 或环境内自签 CA 处理；公网 CA 只是可选情况。自签/企业 CA 是一等支持路径，不要求公网证书，但不能只在 PAAP namespace 内信任证书。
- PAAP 在 registry/Harbor 工作台提供 CA 证书下载入口，并展示证书下载地址：`/api/v1/environments/:id/services/:serviceId/registry-ca.crt`。下载的是 public CA/tls cert，不暴露私钥。用户需要把这个链接、registry host、containerd/Docker 配置路径一起交给集群管理员。
- 节点运行时预先信任企业 CA，或由集群运维统一管理 containerd/Docker 的 registry trust 配置。需要配置的是每个会拉业务镜像的节点，不只是 PAAP server/operator Pod。
- PAAP 只负责生成正确的环境级 registry host，并把 registry/Harbor Helm values 中的 TLS host、Harbor `externalURL` 与该 host 对齐。

正常集群不应把 PAAP 设计成“必须改节点运行时才能部署”的系统。更稳的做法是：

- 云厂商/托管 Kubernetes：使用厂商镜像仓库或企业统一镜像仓库，节点镜像、MachineConfig、启动脚本或节点池配置预置信任；PAAP 不在业务安装流程里触碰节点。
- 自管集群：由集群运维把企业 CA 和 registry mirror/trust 写入节点基线，节点重启、滚动升级、drain 由运维流程控制。
- PAAP 环境级 registry：`PAAP_REGISTRY_HOST_TEMPLATE` 可以按环境生成不同 host，例如 `registry.{app}-{env}.corp.example.com:5443`；业务镜像、registry/Harbor HTTPS host、Harbor `externalURL` 必须使用同一个节点可访问 host。
- PAAP 内部控制面访问 registry/Harbor/Gitea/Jenkins/ArgoCD 时仍优先使用 `*.svc.cluster.local`，避免把内部 API 调用绑死到外部域名。只有写进 Deployment/ArgoCD/kpack Image 的业务镜像地址必须使用节点运行时可访问、可信任的 HTTPS host。
- registry/Harbor 安装在环境内后，PAAP 工作台会给出证书下载链接、registry host、containerd `hosts.toml` 和 Docker `certs.d` 参考路径。PAAP 不默认修改节点运行时；用户应把这些信息交给集群管理员，由管理员按节点池、MachineConfig、启动脚本或企业节点基线统一配置。
- 内网自签证书是默认支持场景。PAAP 提供的是 public CA/tls cert 下载入口，内部服务间访问继续使用集群内 URL；外部/节点运行时信任链由管理员配置到每个会拉业务镜像的节点。

可选节点 agent：

- `deploy/k8s/paap-node-registry-agent.yaml` 是可选的 privileged DaemonSet，用于 kind、裸金属或自管节点。
- 它读取 ConfigMap 的 `registries.txt`，支持多个环境/多个 registry。每行格式是 `registry-host[:port]|https://registry-host[:port]|optional-ca-file`。
- 它把每个 registry 的 endpoint 和 CA 写入节点的 `/etc/containerd/certs.d/<host>/hosts.toml` 与 `/etc/docker/certs.d/<host>/ca.crt`。
- 它默认不重启 containerd/Docker。生产环境应通过节点维护流程 drain/restart，避免 agent 擅自重启运行时造成业务中断。
- 它只能解决 kubelet/containerd/Docker 拉业务镜像的信任问题，不能自动解决 kpack `build-init` 容器内 Go TLS 对私有 CA 的信任问题。
- 它不是生产默认路径。只有在 kind、自管裸金属、临时测试集群，且无法通过节点镜像/节点池/MachineConfig 预置信任时，才建议启用。

## CI 构建方案

不要默认使用 Jenkins agent 内的 `docker build/docker push`：

- kind 节点和 Jenkins agent 不一定有 Docker daemon。
- 挂宿主 Docker socket 有权限和隔离风险。
- 用户源码仓库不一定有 Dockerfile。

默认推荐方案是 Buildpacks-first，但不是在 Jenkins 中直接跑 `pack build`。`pack build` 仍依赖 Docker/兼容 daemon 来运行构建容器，不适合作为离线 kind 默认路径。

当前默认路径：

1. 创建 source 交付组件时，PAAP 创建/更新 Gitea 组件仓库、部署清单、Jenkinsfile 和 ArgoCD Application。
2. PAAP 自动创建/更新 Jenkins Pipeline Job。Job 从 Gitea 仓库的 `components/<component>/Jenkinsfile` 读取流水线定义。
3. PAAP 自动在 Gitea 仓库创建 push webhook，指向 Jenkins `git/notifyCommit`，push 后由 Jenkins SCM 触发构建。
4. source 交付的版本标签可以留空。PAAP 会用 `manual` 作为首次 GitOps 占位标签，但初始 Deployment 副本数会写成 `0`，避免 ArgoCD 在 Jenkins 构建完成前拉取不存在的占位镜像。Jenkins 构建完成后会提交更新后的部署清单并 push 到环境 Gitea 仓库。
5. 初次创建 source 组件时，部署清单和 Jenkinsfile 的提交发生在 Job/Webhook 完成之前，可能错过第一次 webhook。因此 PAAP 在 kpack 环境 ready 且组件处于 `planned`/`pending` 初始状态时，会主动触发一次 Jenkins build；后续同步不重复触发，继续依赖 Gitea webhook/SCM polling。
6. PAAP 同步 Gitea 文件时会比较已有内容。内容未变化时不提交新 commit，避免普通重新同步触发无意义 Jenkins 构建。
7. Jenkins 使用 Kubernetes agent 启动一个带 `kubectl` 的临时 Pod，ServiceAccount 为 `paap-kpack-build`，生成并提交 `kpack.io/Image`。这条路径不依赖 Jenkins controller 或默认 agent 预装 `kubectl`。
8. kpack 控制器在集群内使用 Cloud Native Buildpacks 构建镜像并推送到 registry。
9. Jenkins 等待 kpack Image Ready。
10. Jenkins 更新 `components/<component>/deployment.yaml` 中的镜像标签和副本数，提交并 push 到环境 Gitea 仓库。
11. ArgoCD 监控该 GitOps 仓库变化并自动同步到业务 namespace。

如果 Jenkins 服务还没有安装或暂时不可达，组件的 GitOps/ArgoCD 文件仍会生成，组件 `pipelineStatus` 保持 `pending` 并记录 Jenkins 同步 warning；安装 Jenkins 后可通过 GitOps 重新同步动作补齐 Job/Webhook。

PAAP 轻量 `registry:2` 模板已经切到 HTTPS，Harbor 预设也启用 HTTPS。这里仍有一个重要边界：kpack v0.17.0 的 controller/build-init 使用 go-containerregistry 默认 registry client，未暴露 PAAP 可配置的 insecure HTTP registry 开关；dockerconfig secret 只能解决凭据，不能解决 TLS 信任。

source 组件的构建目标仓库按环境动态选择，不再假设固定 host：

- 如果同环境已经安装并运行 Harbor，PAAP 优先把 source 组件镜像规划到 Harbor。
- 如果 Harbor 不存在，则使用同环境 registry。
- 如果两者都还没安装，则保留默认 registry host 模板作为规划结果，但 CI 会因为没有实际可用的环境 registry/Harbor 而保持 `pending` 或给出 warning。
- 选择轻量 registry 时，registry 本身可以作为 source 构建目标；但仍要求 kpack CRD/controller ready，并且节点运行时信任该 registry 的 HTTPS 证书。kpack controller 与 build pod 的 registry CA 由 PAAP 自动同步。
- 选择 Harbor 时，镜像路径项目名使用环境 primary namespace，例如 `shop-dev/orders-api:<tag>`。PAAP 会在组件创建时调用 Harbor API 自动准备 `shop-dev` project；如果 Harbor project 创建失败，组件创建会明确失败，避免后续 kpack push 阶段才暴露问题。
- PAAP 内部调用 Harbor API 使用集群内 `harbor-core.<namespace>.svc.cluster.local` 这类 URL；写入组件镜像、kpack Image 和 Deployment 的 registry host 仍来自 `PAAP_REGISTRY_HOST_TEMPLATE`，必须是节点运行时可访问并信任的 HTTPS 地址。

要跑通完整 source -> buildpacks -> image push -> ArgoCD 部署链路，registry 必须同时满足：

- Jenkins/kpack 能创建 `kpack.io/Image` 并访问源码。
- kpack controller、build-init、lifecycle/build pod 能信任 registry 的 TLS 证书。内网自签 CA 场景下，PAAP 在 source 组件同步时会从同环境 registry/Harbor TLS Secret 读取 `ca.crt` 或 `tls.crt`，把 CA 同步到 `kpack` namespace，patch `kpack-controller` Deployment 挂载该 CA，同时创建 `paap-kpack-registry-ca` CNB binding Secret，并在 Jenkinsfile 生成的 `kpack.io/Image` 中挂载到 `spec.build.cnbBindings`。如果 PAAP 读不到证书，组件会保留 CI warning，用户应检查 registry/Harbor 是否已安装以及 TLS Secret 是否存在。
- 节点运行时能解析并信任同一个 registry host，ArgoCD 同步后的 Pod 能拉到镜像。

如果 `PAAP_REGISTRY_HOST_TEMPLATE` 仍是默认 `*.paap.local` 占位域名，PAAP 会保留 kpack registry warning，不把 CI 状态误报成完全可用。正常集群应把该模板改成真实 HTTPS registry 域名；私有 CA 场景下，PAAP 自动同步 kpack controller/build pod 信任，节点运行时信任仍由管理员按节点池或节点基线配置。

Dockerfile 构建只作为显式选择：当用户明确提供 Dockerfile 构建策略时，再切到 Kaniko/BuildKit 这类 daemonless Dockerfile 构建器。默认不要求 Dockerfile。

## 离线 CI 还需要补齐的前置

kpack/Buildpacks 路径也需要镜像预加载，包括：

- kpack v0.17.0 release manifest 中的 controller/webhook/build-init/build-waiter/rebase/completion/lifecycle digest 镜像。
- build image：`paketobuildpacks/build-jammy-base:0.1.233`。
- run image：`paketobuildpacks/run-jammy-base:0.1.233`。
- buildpack 镜像：`paketobuildpacks/java:22.0.0`、`paketobuildpacks/nodejs:10.3.2`、`paketobuildpacks/go:4.19.14`、`paketobuildpacks/python:2.49.0`。
- Jenkins controller/agent 镜像
- Jenkinsfile 中执行 Kubernetes 操作的 `docker.io/alpine/k8s:1.34.1`

这些镜像已加入 `deploy/k8s/kind-images.txt` 的基础清单。后续如果替换 stack/buildpack 或升级 kpack，需要同步更新清单并重新执行 `make preload-kind-images`。

`deploy/k8s/deploy.sh` 会在 PAAP server/operator 启动前安装 `deploy/k8s/kpack-v0.17.0.yaml`，也可以单独执行 `make install-kpack`。PAAP 会在 source 组件同步时自动创建 Jenkinsfile 中引用的 `paap-kpack-build` ServiceAccount/RBAC、registry dockerconfig secret、可选的 `paap-kpack-registry-ca` CNB binding Secret、`paap-stack` ClusterStack 和 `paap-builder` Builder；读到 registry/Harbor CA 时，还会把 CA 同步到 `kpack` namespace 并 patch `kpack-controller` Deployment。Jenkinsfile 显式声明 Kubernetes agent pod，使用 `docker.io/alpine/k8s:1.34.1` 和 `paap-kpack-build` 执行 `kubectl apply/wait`，因此 Jenkins controller 镜像不需要内置 kubectl。Builder order 直接引用固定 Paketo buildpack 镜像，每种语言是独立可选 group，不要求一个源码同时满足 Java/Node/Go/Python。若 kpack CRD/controller 缺失，或构建目标仍是 PAAP 轻量 HTTP registry，Jenkins Job/Webhook 会继续同步，但组件 `pipelineStatus` 会保持 `pending` 并记录对应 warning。

## 动态权限边界

所有通过 PAAP 安装的工具、中间件、数据库都走同一个 `ServiceInstance` CR 权限模型，不是只有“工具”才需要控制权限。

权限按模板的 `platform-manifest.yaml` 声明分成三类：

- `toolNamespace`：只绑定到该实例自己的工具 namespace，例如 `app-dev-redis`、`app-dev-git`。数据库和中间件默认主要使用这一类。
- `workloadNamespaces`：只投射到业务负载 namespace。ArgoCD、Jenkins 这类需要控制业务资源的工具可以在这里声明 `*/*/*`。
- `environmentNamespaces`：投射到同环境非自身 namespace，包含业务、工具、中间件、数据库 namespace。Monitor、Loki 这类需要观察整个环境的工具使用这一类。

`ServiceInstanceReconciler` 会 watch 同环境 namespace 的变化，并在 reconcile 时重新发现 `paap.io/app` + `paap.io/env` 相同的 namespace 集合：

- 新增 namespace：按 ServiceInstance 的 `workloadRole` / `environmentRole` 分别补 Role/RoleBinding，并刷新 Helm values 中的 `envNamespaces`。这里的 environment 集合包含业务 namespace 和同环境其它工具 namespace，适配监控、日志这类需要看整个环境的服务。
- 删除 namespace 或删除服务实例：清理同环境 namespace 中残留的 PAAP Role/RoleBinding，包含曾经投射到其它工具 namespace 的 RBAC。
- 只声明 `toolNamespace` 的实例：不会在环境 namespace 中创建 workload/environment RoleBinding，因此数据库/中间件不会获得跨 namespace 权限。

需要注意：如果某个数据库或中间件未来要主动读取/操作业务 namespace，比如自动注入 Secret、生成 ServiceMonitor、扫描 Pod，就必须在自己的 `platform-manifest.yaml` 中显式声明 `workloadNamespaces` 或 `environmentNamespaces` 以及最小权限规则。
