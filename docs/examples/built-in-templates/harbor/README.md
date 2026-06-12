# harbor 内置模板

这是 PAAP 平台的内置模板，使用标准格式。

## 结构

```
harbor/
├── chart/                      # Helm Chart
├── platform-manifest.yaml      # 平台元数据
├── preset-values.yaml          # 配置覆盖（可选）
└── README.md                   # 本文件
```

## 使用

此模板在平台部署时会自动上传到 MinIO，用户可以直接在 UI 中安装。

预设默认使用 HTTPS ingress。PAAP 安装时会把环境 runtime registry host 注入 Harbor 的 `externalURL`、ingress host 和 TLS commonName。
生产集群需要把 `PAAP_REGISTRY_HOST_TEMPLATE` 配置成真实可解析、可被节点和 kpack 信任的 registry 域名模板；不要使用 `*.svc.cluster.local` 作为业务镜像地址。

## 参考

- [自定义模板开发指南](../../design/custom-template-guide.md)
- [模板系统总览](../../design/template-system-overview.md)
- [内置模板设置指南](../../BUILT-IN-TEMPLATES-SETUP.md)
