# eureka 内置模板

这是 PAAP 平台的内置中间件模板，提供 Eureka 服务注册中心的单实例部署。

## 结构

```
eureka/
├── chart/
├── platform-manifest.yaml
├── preset-values.yaml
└── README.md
```

此模板在平台部署时会自动打包并上传到 MinIO，用户可以直接在服务目录中安装。
