GO_VERSION := go1.25.7
SHELL := /bin/bash
GOPATH := $(shell source ~/.gvm/scripts/gvm && gvm use $(GO_VERSION) >/dev/null && go env GOPATH)
CONTROLLER_GEN := $(GOPATH)/bin/controller-gen
SERVER_IMAGE ?= paap-server:v0.1.437
OPERATOR_IMAGE ?= paap-operator:v0.1.52

.PHONY: run run-operator build build-operator test frontend-test frontend-build frontend-smoke frontend-verify verify clean deps fmt lint manifests generate install uninstall install-kpack uninstall-kpack package-built-in-templates preload-kind-images verify-server-image

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

# 运行前端单元测试
frontend-test:
	cd frontend && npm run test

# 构建前端
frontend-build:
	cd frontend && npm run build

# 无 Xorg/headful browser 的前端烟测
frontend-smoke:
	cd frontend && npm run smoke:headless

# 前端完整验证：单测、类型检查、构建、headless smoke
frontend-verify:
	cd frontend && npm run verify

# 后端与前端完整验证
verify: test frontend-verify

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

# 安装 kpack CRD/controller/webhook，供 source 组件 Buildpacks 构建使用
install-kpack:
	kubectl apply -f deploy/k8s/kpack-v0.17.0.yaml
	kubectl get pods -n kpack -o wide
	kubectl get events -n kpack --sort-by=.lastTimestamp | tail -20

# 卸载 kpack
uninstall-kpack:
	kubectl delete -f deploy/k8s/kpack-v0.17.0.yaml --ignore-not-found

# ========== Docker ==========

package-built-in-templates:
	./scripts/package-built-in-templates.sh

preload-kind-images:
	./scripts/preload-kind-images.sh

# 构建 Server 镜像
docker-build-server: package-built-in-templates
	docker build --build-arg FRONTEND_CACHE_BUST="$$(date +%s)" -t $(SERVER_IMAGE) -f Dockerfile.server .
	$(MAKE) verify-server-image SERVER_IMAGE=$(SERVER_IMAGE)

verify-server-image:
	docker run --rm --entrypoint sh $(SERVER_IMAGE) -c 'test -x /paap-server && ls -l /paap-server'

# 构建 Operator 镜像
docker-build-operator:
	docker build -t $(OPERATOR_IMAGE) -f Dockerfile.operator .
