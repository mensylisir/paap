# KEDA 与 KubeVirt 平台插件安装方案

## 结论

KEDA 和 KubeVirt 不应该按“应用组件”或“环境中间件”安装，它们属于集群级平台能力。

| 能力 | 安装范围 | 默认是否安装 | 平台定位 |
| --- | --- | --- | --- |
| HPA | Kubernetes 原生能力 | 默认支持 | 基础弹性伸缩能力，依赖 metrics-server |
| KEDA | 每个 Kubernetes 集群一套 | 可选安装 | 事件驱动伸缩增强能力 |
| KubeVirt | 每个 Kubernetes 集群一套 | 不默认安装 | 虚拟机服务能力，依赖节点虚拟化、存储和网络条件 |
| CDI | 每个 Kubernetes 集群一套 | 跟随 KubeVirt 可选安装 | KubeVirt 镜像导入、PVC/DataVolume 管理能力 |

平台应该把 KEDA、KubeVirt、CDI 归类为“集群插件 / 平台能力”，由平台管理员在 PAAP 安装阶段或平台后台显式启用。业务用户在环境画布、服务目录和组件右侧栏里使用这些能力，但不负责安装这些 operator。

## 设计原则

1. **集群级能力不混入应用生命周期**
   - KEDA、KubeVirt 都会安装 CRD、controller/operator。
   - 它们的生命周期应该跟集群绑定，而不是跟某个应用、某个环境绑定。
   - 应用环境只引用这些能力，例如创建 `HorizontalPodAutoscaler`、`ScaledObject`、`VirtualMachine` 或 `DataVolume`。

2. **安装与使用分离**
   - 安装入口：平台安装脚本或“平台能力/集群插件”管理页。
   - 使用入口：组件右侧栏 `伸缩` tab、服务目录虚拟机服务、环境画布右键菜单。

3. **先探测能力，再开放功能**
   - 后端启动和定时巡检时探测 CRD/operator 状态。
   - 前端根据能力状态控制 UI。
   - 未安装时不报假成功，不生成不可用资源。

4. **KubeVirt 默认不自动安装**
   - KubeVirt 对内核虚拟化、节点权限、存储类、网络插件都有要求。
   - kind、家用开发环境、部分托管 Kubernetes 环境可能无法真正运行 VM。
   - 应由平台管理员确认集群条件后显式开启。

## 安装时机

### 方案 A：PAAP 安装阶段启用

适合生产、测试环境。

```bash
paap install \
  --enable-metrics-server=true \
  --enable-keda=true \
  --enable-kubevirt=false \
  --enable-cdi=false
```

推荐默认值：

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--enable-metrics-server` | `true` | HPA 的基础依赖，开发/测试环境建议默认装 |
| `--enable-keda` | `false` | 事件驱动伸缩能力，管理员按需开启 |
| `--enable-kubevirt` | `false` | 虚拟机能力，不建议默认开启 |
| `--enable-cdi` | `false` | KubeVirt 镜像导入能力，通常跟 KubeVirt 一起开启 |

安装阶段做的事情：

1. 安装 PAAP 基础组件。
2. 根据安装参数安装集群插件。
3. 等待插件 CRD 和 operator 就绪。
4. 写入平台能力状态。
5. 启动 PAAP server。

### 方案 B：PAAP 后台动态安装

适合开发阶段或管理员后续扩容平台能力。

建议新增菜单：

```text
平台服务
  集群插件
    metrics-server
    KEDA
    KubeVirt
    CDI
```

管理员可以执行：

| 操作 | 行为 |
| --- | --- |
| 安装 | 创建 namespace、安装 Helm chart 或官方 manifest、等待 CRD/operator 就绪 |
| 升级 | 按版本升级 chart/operator |
| 禁用 | 禁止新建相关资源，但不删除已有业务资源 |
| 卸载 | 高危操作，需要确认没有依赖资源 |
| 检查 | 重新探测 CRD、Deployment、Webhook、APIService 状态 |

后台动态安装必须走平台插件安装流程，不走普通应用部署流程。

## 安装方式

### metrics-server

HPA 依赖 `metrics.k8s.io`，PAAP 支持 HPA 前必须检查 metrics-server。

探测：

```bash
kubectl get apiservice v1beta1.metrics.k8s.io
kubectl get --raw /apis/metrics.k8s.io/v1beta1/nodes
```

开发环境 kind 常见安装方式：

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl -n kube-system patch deployment metrics-server \
  --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

生产环境不要默认加 `--kubelet-insecure-tls`，应按集群证书配置处理。

### KEDA

推荐使用 Helm 安装：

```bash
helm repo add kedacore https://kedacore.github.io/charts
helm repo update

helm upgrade --install keda kedacore/keda \
  --namespace keda \
  --create-namespace \
  --version <keda-chart-version>
```

PAAP 能力探测：

```bash
kubectl get crd scaledobjects.keda.sh
kubectl get crd triggerauthentications.keda.sh
kubectl -n keda get deploy keda-operator
kubectl -n keda get deploy keda-operator-metrics-apiserver
```

KEDA 就绪后，组件右侧栏 `伸缩` tab 才允许选择事件驱动伸缩策略。

### KubeVirt

推荐使用官方 operator 安装，不建议作为普通 Helm 应用塞进环境画布。

安装示例：

```bash
export KUBEVIRT_VERSION=v1.4.0

kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml
```

PAAP 能力探测：

```bash
kubectl get crd virtualmachines.kubevirt.io
kubectl get crd virtualmachineinstances.kubevirt.io
kubectl -n kubevirt get kubevirt kubevirt
kubectl -n kubevirt get deploy virt-operator virt-api virt-controller
kubectl -n kubevirt get daemonset virt-handler
```

节点能力探测：

```bash
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.allocatable.devices\.kubevirt\.io/kvm}{"\n"}{end}'
```

如果没有 KVM 设备或集群策略不允许特权组件，PAAP 应标记 KubeVirt 为 `Degraded` 或 `Unavailable`，服务目录里的虚拟机创建入口置灰。

### CDI

CDI 用于 KubeVirt 镜像导入、DataVolume 和 PVC 管理。

安装示例：

```bash
export CDI_VERSION=v1.61.0

kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml
kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml
```

PAAP 能力探测：

```bash
kubectl get crd datavolumes.cdi.kubevirt.io
kubectl -n cdi get cdi cdi
kubectl -n cdi get deploy cdi-operator cdi-apiserver cdi-uploadproxy cdi-deployment
```

## PAAP 功能开关行为

### HPA

| 状态 | UI 行为 | 后端行为 |
| --- | --- | --- |
| metrics-server 可用 | `伸缩` tab 允许启用 HPA | 生成并应用 `HorizontalPodAutoscaler` |
| metrics-server 不可用 | HPA 表单置灰或提示依赖缺失 | 拒绝保存 HPA，返回明确错误 |

### KEDA

| 状态 | UI 行为 | 后端行为 |
| --- | --- | --- |
| KEDA 可用 | `伸缩` tab 允许启用 KEDA ScaledObject | 生成 `ScaledObject` 和必要的 `TriggerAuthentication` |
| KEDA 未安装 | KEDA 选项置灰，提示安装 KEDA | 拒绝保存 KEDA 配置 |
| KEDA 异常 | 展示 `Degraded` 状态和错误信息 | 禁止新建，保留已有资源只读展示 |

### KubeVirt

| 状态 | UI 行为 | 后端行为 |
| --- | --- | --- |
| KubeVirt + CDI 可用 | 服务目录虚拟机服务可创建，画布右键显示“虚拟机” | 创建 VM/DataVolume/Service |
| 只有 KubeVirt，无 CDI | 允许使用已有 PVC 创建 VM，镜像导入入口置灰 | 禁止 DataVolume 导入 |
| KubeVirt 未安装 | 虚拟机服务展示文档，创建入口置灰 | 拒绝创建 VM |
| 节点无 KVM | 标记不可用或 degraded | 禁止创建需要硬件虚拟化的 VM |

## 数据模型建议

建议增加集群插件状态表，避免每个请求都直接查 Kubernetes API。

```sql
CREATE TABLE cluster_addons (
  id BIGSERIAL PRIMARY KEY,
  cluster_id BIGINT NOT NULL,
  name VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  category VARCHAR(64) NOT NULL,
  namespace VARCHAR(128) NOT NULL,
  version VARCHAR(64) NOT NULL DEFAULT '',
  desired_state VARCHAR(32) NOT NULL DEFAULT 'disabled',
  status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  install_method VARCHAR(32) NOT NULL DEFAULT 'helm',
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  conditions JSONB NOT NULL DEFAULT '[]'::jsonb,
  error_message TEXT NOT NULL DEFAULT '',
  installed_at TIMESTAMPTZ NULL,
  last_checked_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (cluster_id, name)
);

CREATE INDEX idx_cluster_addons_cluster_status
  ON cluster_addons (cluster_id, status);

CREATE INDEX idx_cluster_addons_name_status
  ON cluster_addons (name, status);
```

字段约定：

| 字段 | 示例 | 说明 |
| --- | --- | --- |
| `name` | `keda`、`kubevirt`、`cdi`、`metrics-server` | 插件唯一标识 |
| `desired_state` | `enabled`、`disabled` | 管理员期望状态 |
| `status` | `available`、`installing`、`degraded`、`unavailable`、`unknown` | 实际状态 |
| `conditions` | CRD/operator/webhook/APIService 检查结果 | 前端详情页可直接展示 |
| `config` | chart values、版本、安装参数 | 安装/升级时使用 |

## API 设计建议

### 查询平台能力

```http
GET /api/v1/platform/capabilities
```

响应：

```json
{
  "data": {
    "hpa": {
      "available": true,
      "reason": "",
      "dependencies": ["metrics-server"]
    },
    "keda": {
      "available": false,
      "reason": "scaledobjects.keda.sh CRD not found",
      "addonStatus": "unavailable"
    },
    "kubevirt": {
      "available": false,
      "reason": "virtualmachines.kubevirt.io CRD not found",
      "addonStatus": "unavailable"
    },
    "cdi": {
      "available": false,
      "reason": "datavolumes.cdi.kubevirt.io CRD not found",
      "addonStatus": "unavailable"
    }
  }
}
```

### 查询集群插件

```http
GET /api/v1/platform/addons
```

响应：

```json
{
  "data": [
    {
      "name": "keda",
      "displayName": "KEDA",
      "namespace": "keda",
      "version": "2.17.0",
      "desiredState": "enabled",
      "status": "available",
      "lastCheckedAt": "2026-06-30T10:00:00+08:00",
      "conditions": [
        {
          "type": "CRDReady",
          "status": "True",
          "message": "scaledobjects.keda.sh found"
        },
        {
          "type": "OperatorReady",
          "status": "True",
          "message": "keda-operator available"
        }
      ]
    }
  ]
}
```

### 安装或升级插件

```http
POST /api/v1/platform/addons/keda/install
Content-Type: application/json
```

请求：

```json
{
  "version": "2.17.0",
  "namespace": "keda",
  "values": {
    "metricsServer": {
      "useHostNetwork": false
    }
  }
}
```

响应：

```json
{
  "data": {
    "name": "keda",
    "status": "installing",
    "message": "KEDA installation started"
  }
}
```

## 页面与流程落点

### 平台能力页

新增或复用 `平台服务` 下的能力管理页面：

```text
平台服务 / 集群插件
```

页面展示：

| 插件 | 状态 | 版本 | 命名空间 | 依赖 | 操作 |
| --- | --- | --- | --- | --- | --- |
| metrics-server | Available | v0.7.x | kube-system | 无 | 检查/升级 |
| KEDA | Unavailable | - | keda | metrics-server 可选 | 安装 |
| KubeVirt | Unavailable | - | kubevirt | KVM、存储类 | 安装 |
| CDI | Unavailable | - | cdi | KubeVirt | 安装 |

### 组件右侧栏伸缩 tab

组件右侧栏 `伸缩` tab 中：

1. HPA 区块永远展示，但依赖不可用时禁用保存。
2. KEDA 区块只有在能力可用时允许配置。
3. 能力不可用时展示明确原因和安装入口。

### 服务目录虚拟机服务

服务目录可以展示 KubeVirt 虚拟机模板、概览和 Quick Start。

如果 KubeVirt 未安装：

- 服务详情文档仍可阅读。
- 创建按钮置灰。
- 显示“需要管理员安装 KubeVirt 集群插件”。

如果 KubeVirt 已安装但 CDI 未安装：

- 支持已有 PVC 场景。
- 禁止镜像上传/导入 DataVolume 场景。

## 推荐默认策略

| 环境 | metrics-server | KEDA | KubeVirt | CDI |
| --- | --- | --- | --- | --- |
| 本地 kind 开发 | 可安装 | 可选 | 默认不装 | 默认不装 |
| 公司测试集群 | 安装 | 推荐安装 | 按需安装 | 跟随 KubeVirt |
| 生产集群 | 安装 | 按业务需要安装 | 管理员评估后安装 | 跟随 KubeVirt |

KEDA 比较轻，适合作为平台增强能力提供一键安装。

KubeVirt 不适合默认安装，必须先确认：

1. 节点支持硬件虚拟化。
2. 集群安全策略允许 KubeVirt 所需权限。
3. 存储类支持 VM 磁盘场景。
4. 网络模型满足 VM 访问需求。
5. 平台管理员接受 operator 带来的运维复杂度。

## 后续实现 TODO

1. 新增 `cluster_addons` 或 `platform_capabilities` 表。
2. 后端增加集群能力探测服务：
   - metrics-server / HPA
   - KEDA CRD 和 operator
   - KubeVirt CRD、operator、virt-handler、KVM 设备
   - CDI CRD 和 operator
3. 增加 `/api/v1/platform/capabilities` 和 `/api/v1/platform/addons` API。
4. `伸缩` tab 接入能力状态：
   - HPA 依赖 metrics-server。
   - KEDA 依赖 KEDA addon available。
5. 服务目录和画布右键菜单接入 KubeVirt 能力状态。
6. 增加平台插件管理页，支持安装、升级、检查和禁用。
7. 安装脚本增加 `--enable-keda`、`--enable-kubevirt`、`--enable-cdi` 参数。
8. kind 开发脚本只默认处理 metrics-server，KubeVirt 保持手动开启。

