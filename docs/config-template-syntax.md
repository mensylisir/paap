# 配置模板语法

PAAP 配置模板以原生配置文件为主，支持 YAML、JSON、TOML、properties、env、ini、Nginx/conf 和普通文本。普通用户不需要手写 PAAP 的结构化 JSON；只需要在原生配置中用 `__TEMPLATE__` 标记需要替换的值。

## 普通语法

普通变量：

```yaml
spring:
  datasource:
    url: __TEMPLATE__JDBC_URL__数据库地址__
    username: __TEMPLATE__JDBC_USER__数据库用户__
```

带默认值：

```nginx
listen __TEMPLATE__LISTEN_PORT__监听端口__DEFAULT__80__;
```

语法规则：

```text
__TEMPLATE__KEY__显示名__
__TEMPLATE__KEY__显示名__DEFAULT__默认值__
```

PAAP 会自动扫描这些标记并生成字段定义。字段类型会根据 KEY 推断：

- `PASSWORD`、`SECRET`、`TOKEN`、`PRIVATE_KEY`、`ACCESS_KEY` 推断为敏感字段。
- `PORT` 推断为数字。
- `JDBC`、`DATABASE`、`POSTGRES`、`MYSQL` 和 `URL/HOST/ADDR` 组合会推荐数据库绑定。
- `REDIS`、`RABBITMQ`、`KAFKA`、`MINIO/S3` 会推荐对应中间件绑定。

## 循环块

循环块用于 Nginx 多个 location、多个 upstream、多个 topic 等重复片段。

```nginx
__TEMPLATE__FOR__LOCATION_LIST__位置块列表__
location __TEMPLATE__ITEM_API_PATH__匹配路径__ {
  proxy_pass __TEMPLATE__ITEM_PROXY_PASS__路由转发__;
}
__TEMPLATE__END__LOCATION_LIST__
```

规则：

```text
__TEMPLATE__FOR__LIST_KEY__显示名__
  __TEMPLATE__ITEM_FIELD_KEY__显示名__
__TEMPLATE__END__LIST_KEY__
```

PAAP 会把 `LIST_KEY` 解析成列表字段，把 `ITEM_` 开头的变量解析成列表项字段。普通用户只维护原生配置片段，不需要手写 list schema。

## 条件块

条件块用于“启用 Redis 才生成 Redis 配置”这类可选配置。

```yaml
__TEMPLATE__IF__REDIS_ENABLED__启用 Redis__
redis:
  host: __TEMPLATE__REDIS_HOST__Redis 地址__
  port: __TEMPLATE__REDIS_PORT__Redis 端口__DEFAULT__6379__
__TEMPLATE__END__REDIS_ENABLED__
```

规则：

```text
__TEMPLATE__IF__FLAG_KEY__显示名__
  ...
__TEMPLATE__END__FLAG_KEY__
```

PAAP 会把 `FLAG_KEY` 解析成布尔字段。

## 高级语法

平台工程师可以导入高级 JSON / schema，用于精确控制字段类型、默认值、服务绑定、敏感字段、循环项、条件块、环境变量、启动参数和生成文件。

示例：

```json
{
  "name": "Spring Boot + PostgreSQL",
  "framework": "springboot",
  "componentTypes": ["backend"],
  "fields": [
    {
      "key": "JDBC_URL",
      "label": "数据库地址",
      "type": "serviceRef",
      "target": "postgresql|mysql",
      "format": "jdbcUrl",
      "required": true
    }
  ],
  "configMaps": [
    {
      "data": {
        "application-paap.yml": "spring:\n  datasource:\n    url: [[paap:JDBC_URL]]\n"
      }
    }
  ],
  "files": [
    {
      "key": "application-paap.yml",
      "recommendedMountPath": "/etc/paap/application-paap.yml",
      "readOnly": true
    }
  ]
}
```

高级 JSON 是内部表达和批量迁移格式，不是普通用户的主要入口。

## 挂载路径

配置模板只声明推荐路径，例如：

```json
{
  "key": "application-paap.yml",
  "recommendedMountPath": "/etc/paap/application-paap.yml"
}
```

实际挂载路径属于组件运行时部署决策。组件右侧栏应用模板时会使用推荐值自动填充，用户需要覆盖时在具体组件的配置文件高级区域调整。

## 渲染与校验

PAAP 的模板引擎分三层：

- 通用文本扫描器：扫描 `__TEMPLATE__` 标记并生成字段。
- schema 推断器：根据 KEY 推断类型、敏感性和服务绑定建议。
- 格式适配器：按 YAML、JSON、TOML、env、ini、Nginx/conf 等格式做渲染校验和必要转义。

模板导入时必须检查未闭合的 `FOR/IF`、重复字段、非法标记和渲染后的格式错误。
