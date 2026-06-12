# jenkins 内置模板

这是 PAAP 平台的内置模板，使用标准格式。

## 结构

```
jenkins/
├── chart/                      # Helm Chart
├── platform-manifest.yaml      # 平台元数据
├── preset-values.yaml          # 配置覆盖（可选）
└── README.md                   # 本文件
```

## 使用

此模板在平台部署时会自动上传到 MinIO，用户可以直接在 UI 中安装。

模板源码位于 `docs/examples/built-in-templates/jenkins/`，发布时通过 `scripts/package-built-in-templates.sh` 打包为 `data/charts/jenkins.tar.gz`。运行时 `paap-server` 镜像把 chart 包放在 `/charts/jenkins.tar.gz`，init job 上传到 MinIO 的 `paap-charts/charts/jenkins.tar.gz`，安装 Jenkins 时从 MinIO 下载该 chart 包。

## CI 构建策略

不要默认假设 kind 集群或 Jenkins agent 里有 Docker daemon，也不要默认要求业务仓库有 Dockerfile。PAAP 默认采用 Buildpacks-first 方案：Jenkins 负责编排 kpack `Image` 资源，由 kpack 在集群内完成 Cloud Native Buildpacks 构建并推送镜像。Dockerfile 构建只作为用户显式选择的高级模式。

创建 source 交付组件时，PAAP 会自动创建/更新 Jenkins Pipeline Job，并在 Gitea 组件仓库上创建 push webhook 指向 Jenkins `git/notifyCommit`。用户不需要手工配置 Jenkins Job 或 Gitea webhook。

离线 kind 集群需要先在宿主机拉取并导入 Jenkins、kpack、builder、run image 等镜像，参见 `docs/KIND-OFFLINE-CI-AND-TEMPLATES.md` 和 `deploy/k8s/kind-images.txt`。

## 参考

- [自定义模板开发指南](../../design/custom-template-guide.md)
- [模板系统总览](../../design/template-system-overview.md)
- [内置模板设置指南](../../BUILT-IN-TEMPLATES-SETUP.md)
