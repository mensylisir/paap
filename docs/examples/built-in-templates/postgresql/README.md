# postgresql 内置模板

这是 PAAP 平台的内置模板，使用标准格式。

## 结构

```
postgresql/
├── chart/                      # Helm Chart
├── platform-manifest.yaml      # 平台元数据
├── preset-values.yaml          # 配置覆盖（可选）
└── README.md                   # 本文件
```

## 使用

此模板在平台部署时会自动上传到 MinIO，用户可以直接在 UI 中安装。

## 参考

- [自定义模板开发指南](../../design/custom-template-guide.md)
- [模板系统总览](../../design/template-system-overview.md)
- [内置模板设置指南](../../BUILT-IN-TEMPLATES-SETUP.md)
