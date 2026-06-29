# nacos 内置模板

这是 PAAP 平台的内置中间件模板，提供 Nacos 注册中心与配置中心的 standalone 部署。

## 结构

```
nacos/
├── chart/
├── platform-manifest.yaml
├── preset-values.yaml
└── README.md
```

此模板在平台部署时会自动打包并上传到 MinIO，用户可以直接在服务目录中安装。
