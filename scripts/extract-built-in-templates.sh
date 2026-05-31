#!/bin/bash
# 解压内置模板到 docs/examples/built-in-templates/

set -e

SOURCE_DIR="data/charts"
TARGET_DIR="docs/examples/built-in-templates"

echo "Extracting built-in templates..."
echo "Source: $SOURCE_DIR"
echo "Target: $TARGET_DIR"
echo ""

mkdir -p "$TARGET_DIR"

for tarball in "$SOURCE_DIR"/*.tar.gz; do
    if [ ! -f "$tarball" ]; then
        echo "No templates found in $SOURCE_DIR"
        exit 1
    fi

    template_name=$(basename "$tarball" .tar.gz)
    echo "Processing $template_name..."

    # 创建目标目录
    mkdir -p "$TARGET_DIR/$template_name"

    # 解压
    tar -xzf "$tarball" -C "$TARGET_DIR/$template_name" --strip-components=1 2>/dev/null || \
    tar -xzf "$tarball" -C "$TARGET_DIR/$template_name"

    # 创建 README.md
    cat > "$TARGET_DIR/$template_name/README.md" <<EOF
# $template_name 内置模板

这是 PAAP 平台的内置模板，使用标准格式。

## 结构

\`\`\`
$template_name/
├── chart/                      # Helm Chart
├── platform-manifest.yaml      # 平台元数据
├── preset-values.yaml          # 配置覆盖（可选）
└── README.md                   # 本文件
\`\`\`

## 使用

此模板在平台部署时会自动上传到 MinIO，用户可以直接在 UI 中安装。

## 参考

- [自定义模板开发指南](../../design/custom-template-guide.md)
- [模板系统总览](../../design/template-system-overview.md)
- [内置模板设置指南](../../BUILT-IN-TEMPLATES-SETUP.md)
EOF

    echo "✓ Extracted $template_name"
done

echo ""
echo "All templates extracted to $TARGET_DIR"
echo ""
echo "Next steps:"
echo "1. Review the extracted templates"
echo "2. Deploy MinIO: kubectl apply -f deploy/k8s/minio.yaml"
echo "3. Initialize templates: kubectl apply -f deploy/k8s/init-templates.yaml"
