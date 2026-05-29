GO_VERSION := go1.25.7
GOPATH := $(shell source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go env GOPATH)
CONTROLLER_GEN := $(GOPATH)/bin/controller-gen

.PHONY: run run-operator build build-operator test clean deps fmt lint manifests generate install uninstall

# ========== Server ==========

# 运行开发服务器
run:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go run cmd/server/main.go

# 构建 Server 二进制
build:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go build -o bin/paap-server cmd/server/main.go

# ========== Operator ==========

# 运行 Operator（本地，连接 kind 集群）
run-operator:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go run cmd/operator/main.go

# 构建 Operator 二进制
build-operator:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go build -o bin/paap-operator cmd/operator/main.go

# ========== 通用 ==========

# 构建所有
all: build build-operator

# 运行测试
test:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go test ./...

# 安装依赖
deps:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go mod download && go mod tidy

# 清理构建文件
clean:
	rm -rf bin/

# 代码格式化
fmt:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go fmt ./...

# 代码检查
lint:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && go vet ./...

# ========== CRD ==========

# 生成 CRD YAML 和 deepcopy
manifests: generate
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && $(CONTROLLER_GEN) crd paths="./api/..." output:crd:dir=config/crd/bases

# 生成 deepcopy 函数
generate:
	@source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) && $(CONTROLLER_GEN) object paths="./api/..."

# 安装 CRD 到集群
install: manifests
	kubectl apply -f config/crd/bases/

# 从集群卸载 CRD
uninstall:
	kubectl delete -f config/crd/bases/ --ignore-not-found

# ========== Docker ==========

# 构建 Server 镜像
docker-build-server:
	docker build -t paap-server:latest -f Dockerfile.server .

# 构建 Operator 镜像
docker-build-operator:
	docker build -t paap-operator:latest -f Dockerfile.operator .
