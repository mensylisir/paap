package service

// ArgoCD namespace-scoped deployment template
const argocdTemplate = `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .serviceAccountName }}
  namespace: {{ .toolNamespace }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-manager
  namespace: {{ .toolNamespace }}
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
    resources: ["*"]
    verbs: ["*"]
  - apiGroups: ["argoproj.io"]
    resources: ["applications", "appprojects"]
    verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: argocd-manager
  namespace: {{ .toolNamespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: argocd-manager
subjects:
  - kind: ServiceAccount
    name: {{ .serviceAccountName }}
    namespace: {{ .toolNamespace }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: {{ .toolNamespace }}
data:
  application.namespaces: "{{ join "," .namespaces }}"
  server.insecure: "true"
  url: "http://argocd-server:8080"
  dexserver.disabled: "true"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
  namespace: {{ .toolNamespace }}
data:
  policy.default: "role:admin"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: {{ .toolNamespace }}
data:
  server.insecure: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-redis
  namespace: {{ .toolNamespace }}
  labels:
    app: argocd-redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: argocd-redis
  template:
    metadata:
      labels:
        app: argocd-redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
---
apiVersion: v1
kind: Service
metadata:
  name: argocd-redis
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: argocd-redis
  ports:
    - port: 6379
      targetPort: 6379
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
  namespace: {{ .toolNamespace }}
  labels:
    app: argocd-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: argocd-server
  template:
    metadata:
      labels:
        app: argocd-server
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8083"
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: server
          image: quay.io/argoproj/argocd:{{ index .parameters "version" | default "v2.10" }}
          command:
            - argocd-server
            - --staticassets
            - /shared/app
            - --redis
            - argocd-redis:6379
            - --repo-server
            - argocd-repo-server:8081
            - --insecure
          ports:
            - containerPort: 8080
            - containerPort: 8083
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-repo-server
  namespace: {{ .toolNamespace }}
  labels:
    app: argocd-repo-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: argocd-repo-server
  template:
    metadata:
      labels:
        app: argocd-repo-server
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: repo-server
          image: quay.io/argoproj/argocd:{{ index .parameters "version" | default "v2.10" }}
          command:
            - argocd-repo-server
            - --redis
            - argocd-redis:6379
          ports:
            - containerPort: 8081
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: argocd-application-controller
  namespace: {{ .toolNamespace }}
  labels:
    app: argocd-application-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: argocd-application-controller
  serviceName: argocd-application-controller
  template:
    metadata:
      labels:
        app: argocd-application-controller
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8082"
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: controller
          image: quay.io/argoproj/argocd:{{ index .parameters "version" | default "v2.10" }}
          command:
            - argocd-application-controller
            - --redis
            - argocd-redis:6379
            - --repo-server
            - argocd-repo-server:8081
          env:
            - name: ARGOCD_CONTROLLER_NAMESPACES
              value: "{{ join "," .namespaces }}"
          ports:
            - containerPort: 8082
          resources:
            requests:
              cpu: 200m
              memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: argocd-server
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: argocd-server
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: metrics
      port: 8083
      targetPort: 8083
---
apiVersion: v1
kind: Service
metadata:
  name: argocd-repo-server
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: argocd-repo-server
  ports:
    - port: 8081
      targetPort: 8081
`

// Tekton Pipelines deployment template
const tektonTemplate = `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .serviceAccountName }}
  namespace: {{ .toolNamespace }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-pipeline-role
  namespace: {{ .toolNamespace }}
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "services", "configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "create", "delete"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ci-pipeline-role-binding
  namespace: {{ .toolNamespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ci-pipeline-role
subjects:
  - kind: ServiceAccount
    name: {{ .serviceAccountName }}
    namespace: {{ .toolNamespace }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tekton-pipelines-controller
  namespace: {{ .toolNamespace }}
  labels:
    app: tekton-pipelines-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tekton-pipelines-controller
  template:
    metadata:
      labels:
        app: tekton-pipelines-controller
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: controller
          image: gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.53.0
          ports:
            - containerPort: 9090
            - containerPort: 8080
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
          env:
            - name: SYSTEM_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tekton-pipelines-webhook
  namespace: {{ .toolNamespace }}
  labels:
    app: tekton-pipelines-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tekton-pipelines-webhook
  template:
    metadata:
      labels:
        app: tekton-pipelines-webhook
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: webhook
          image: gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook:v0.53.0
          ports:
            - containerPort: 8443
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: tekton-pipelines-controller
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: tekton-pipelines-controller
  ports:
    - port: 9090
      targetPort: 9090
    - port: 8080
      targetPort: 8080
`

// Prometheus + Grafana deployment template
const prometheusTemplate = `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .serviceAccountName }}
  namespace: {{ .toolNamespace }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prometheus-reader
  namespace: {{ .toolNamespace }}
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: prometheus-reader-binding
  namespace: {{ .toolNamespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: prometheus-reader
subjects:
  - kind: ServiceAccount
    name: {{ .serviceAccountName }}
    namespace: {{ .toolNamespace }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: {{ .toolNamespace }}
  labels:
    app: prometheus
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: prometheus
          image: prom/prometheus:v2.48.0
          ports:
            - containerPort: 9090
          args:
            - "--config.file=/etc/prometheus/prometheus.yml"
            - "--storage.tsdb.retention.time={{ index .parameters "retention" | default "15d" }}"
            - "--web.listen-address=:9090"
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
      volumes:
        - name: config
          configMap:
            name: prometheus-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: {{ .toolNamespace }}
data:
  prometheus.yml: |
    global:
      scrape_interval: 30s
    scrape_configs:
      - job_name: 'pods'
        kubernetes_sd_configs:
          - role: pod
            namespaces:
              names:{{ range .namespaces }}
                - {{ . }}{{ end }}
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
            action: keep
            regex: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: {{ .toolNamespace }}
  labels:
    app: grafana
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grafana
  template:
    metadata:
      labels:
        app: grafana
    spec:
      containers:
        - name: grafana
          image: grafana/grafana:10.2.0
          ports:
            - containerPort: 3000
          env:
            - name: GF_SECURITY_ADMIN_PASSWORD
              value: "admin"
            - name: GF_PATHS_PROVISIONING
              value: /etc/grafana/provisioning
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
          volumeMounts:
            - name: datasource-config
              mountPath: /etc/grafana/provisioning/datasources
            - name: dashboard-provider
              mountPath: /etc/grafana/provisioning/dashboards
            - name: dashboard-volume
              mountPath: /var/lib/grafana/dashboards
      volumes:
        - name: datasource-config
          configMap:
            name: grafana-datasources
        - name: dashboard-provider
          configMap:
            name: grafana-dashboards-provider
        - name: dashboard-volume
          configMap:
            name: grafana-dashboards-k8s
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: prometheus
  ports:
    - port: 9090
      targetPort: 9090
---
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: grafana
  ports:
    - port: 3000
      targetPort: 3000
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-datasources
  namespace: {{ .toolNamespace }}
data:
  datasources.yaml: |
    apiVersion: 1
    datasources:
      - name: Prometheus
        type: prometheus
        url: http://prometheus:9090
        access: proxy
        isDefault: true
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards-provider
  namespace: {{ .toolNamespace }}
data:
  dashboards.yaml: |
    apiVersion: 1
    providers:
      - name: default
        orgId: 1
        type: file
        disableDeletion: false
        updateIntervalSeconds: 30
        options:
          path: /var/lib/grafana/dashboards
          foldersFromFilesStructure: true
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards-k8s
  namespace: {{ .toolNamespace }}
  labels:
    grafana_dashboard: "1"
data:
  k8s-overview.json: |
    {
      "annotations": { "list": [] },
      "editable": true,
      "fiscalYearStartMonth": 0,
      "graphTooltip": 0,
      "id": null,
      "links": [],
      "panels": [
        {
          "title": "CPU Usage by Namespace",
          "type": "timeseries",
          "datasource": { "type": "prometheus", "uid": "" },
          "gridPos": { "h": 8, "w": 12, "x": 0, "y": 0 },
          "targets": [
            {
              "expr": "sum(rate(container_cpu_usage_seconds_total{namespace=~\"$namespace\",container!=\"\"}[5m])) by (namespace)",
              "legendFormat": "{{ raw "{{namespace}}" }}"
            }
          ],
          "fieldConfig": {
            "defaults": { "unit": "percentunit" }
          }
        },
        {
          "title": "Memory Usage by Namespace",
          "type": "timeseries",
          "datasource": { "type": "prometheus", "uid": "" },
          "gridPos": { "h": 8, "w": 12, "x": 12, "y": 0 },
          "targets": [
            {
              "expr": "sum(container_memory_working_set_bytes{namespace=~\"$namespace\",container!=\"\"}) by (namespace)",
              "legendFormat": "{{ raw "{{namespace}}" }}"
            }
          ],
          "fieldConfig": {
            "defaults": { "unit": "bytes" }
          }
        },
        {
          "title": "Pod Count by Namespace",
          "type": "stat",
          "datasource": { "type": "prometheus", "uid": "" },
          "gridPos": { "h": 4, "w": 6, "x": 0, "y": 8 },
          "targets": [
            {
              "expr": "count(kube_pod_info{namespace=~\"$namespace\"}) by (namespace)",
              "legendFormat": "{{ raw "{{namespace}}" }}"
            }
          ]
        },
        {
          "title": "Restart Count (Top 10)",
          "type": "table",
          "datasource": { "type": "prometheus", "uid": "" },
          "gridPos": { "h": 8, "w": 12, "x": 0, "y": 12 },
          "targets": [
            {
              "expr": "topk(10, sum(increase(kube_pod_container_status_restarts_total{namespace=~\"$namespace\"}[1h])) by (namespace, pod))",
              "legendFormat": "{{ raw "{{namespace}}" }}/{{ raw "{{pod}}" }}",
              "instant": true
            }
          ]
        }
      ],
      "templating": {
        "list": [
          {
            "name": "namespace",
            "type": "query",
            "datasource": { "type": "prometheus", "uid": "" },
            "query": "label_values(kube_pod_info, namespace)",
            "refresh": 2,
            "multi": true,
            "includeAll": true,
            "current": { "text": "All", "value": "$__all" }
          }
        ]
      },
      "time": { "from": "now-1h", "to": "now" },
      "title": "K8s Overview",
      "uid": "k8s-overview",
      "version": 1
    }
`

// Loki deployment template
const lokiTemplate = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki
  namespace: {{ .toolNamespace }}
  labels:
    app: loki
spec:
  replicas: 1
  selector:
    matchLabels:
      app: loki
  template:
    metadata:
      labels:
        app: loki
    spec:
      containers:
        - name: loki
          image: grafana/loki:2.9.0
          ports:
            - containerPort: 3100
          args:
            - "-config.file=/etc/loki/local-config.yaml"
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
          volumeMounts:
            - name: config
              mountPath: /etc/loki
      volumes:
        - name: config
          configMap:
            name: loki-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: loki-config
  namespace: {{ .toolNamespace }}
data:
  local-config.yaml: |
    auth_enabled: false
    server:
      http_listen_port: 3100
    common:
      path_prefix: /loki
      storage:
        filesystem:
          chunks_directory: /loki/chunks
          rules_directory: /loki/rules
      replication_factor: 1
      ring:
        kvstore:
          store: inmemory
    schema_config:
      configs:
        - from: 2020-10-24
          store: tsdb
          object_store: filesystem
          schema: v13
          index:
            prefix: index_
            period: 24h
    limits_config:
      retention_period: {{ index .parameters "retention" | default "14d" }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: promtail
  namespace: {{ .toolNamespace }}
  labels:
    app: promtail
spec:
  replicas: 1
  selector:
    matchLabels:
      app: promtail
  template:
    metadata:
      labels:
        app: promtail
    spec:
      serviceAccountName: {{ .serviceAccountName }}
      containers:
        - name: promtail
          image: grafana/promtail:2.9.0
          args:
            - "-config.file=/etc/promtail/config.yml"
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
          volumeMounts:
            - name: config
              mountPath: /etc/promtail
            - name: varlog
              mountPath: /var/log
            - name: containers
              mountPath: /var/lib/docker/containers
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: promtail-config
        - name: varlog
          hostPath:
            path: /var/log
        - name: containers
          hostPath:
            path: /var/lib/docker/containers
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: promtail-config
  namespace: {{ .toolNamespace }}
data:
  config.yml: |
    server:
      http_listen_port: 9080
    positions:
      filename: /tmp/positions.yaml
    clients:
      - url: http://loki:3100/loki/api/v1/push
    scrape_configs:
      - job_name: kubernetes-pods
        kubernetes_sd_configs:
          - role: pod
            namespaces:
              names:{{ range .namespaces }}
                - {{ . }}{{ end }}
---
apiVersion: v1
kind: Service
metadata:
  name: loki
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: loki
  ports:
    - port: 3100
      targetPort: 3100
`

// Docker Registry deployment template (lightweight)
const registryTemplate = `---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-data
  namespace: {{ .toolNamespace }}
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: {{ index .parameters "storage" | default "10Gi" }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: docker-registry
  namespace: {{ .toolNamespace }}
  labels:
    app: docker-registry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: docker-registry
  template:
    metadata:
      labels:
        app: docker-registry
    spec:
      containers:
        - name: registry
          image: registry:2
          ports:
            - containerPort: 5000
          env:
            - name: REGISTRY_STORAGE_DELETE_ENABLED
              value: "true"
          volumeMounts:
            - name: data
              mountPath: /var/lib/registry
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: registry-data
---
apiVersion: v1
kind: Service
metadata:
  name: docker-registry
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: docker-registry
  ports:
    - port: 5000
      targetPort: 5000
`

// KingbaseES deployment template
const kingbaseTemplate = `---
apiVersion: v1
kind: Secret
metadata:
  name: kingbase-secret
  namespace: {{ .toolNamespace }}
type: Opaque
stringData:
  password: {{ index .parameters "password" | default "changeme123" }}
  database: {{ index .parameters "database" | default "appdb" }}
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: kingbase-pvc
  namespace: {{ .toolNamespace }}
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: {{ index .parameters "storage" | default "10Gi" }}
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kingbase
  namespace: {{ .toolNamespace }}
  labels:
    app: kingbase
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kingbase
  serviceName: kingbase
  template:
    metadata:
      labels:
        app: kingbase
    spec:
      containers:
        - name: kingbase
          image: kingbase:v8r6
          ports:
            - containerPort: 54321
          env:
            - name: DB_USER
              value: "system"
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: kingbase-secret
                  key: password
            - name: DB_NAME
              valueFrom:
                secretKeyRef:
                  name: kingbase-secret
                  key: database
          volumeMounts:
            - name: data
              mountPath: /opt/kingbase/data
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: kingbase-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: kingbase
  namespace: {{ .toolNamespace }}
spec:
  selector:
    app: kingbase
  ports:
    - port: 54321
      targetPort: 54321
`
