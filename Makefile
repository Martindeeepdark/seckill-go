GOHOSTOS := $(shell go env GOHOSTOS)
GOPATH   := $(shell go env GOPATH)
PROTOC   ?= protoc

# ── 模块 ────────────────────────────────────────────────────
SHARE_MODULES     := api common
LEAF_SERVICES     := services/activity-service services/stock-service services/risk-service services/order-service services/support-service
CONSUMER_SERVICES := services/seckill-gateway services/seckill-processor services/seckill-job
ALL_SERVICES      := $(LEAF_SERVICES) $(CONSUMER_SERVICES)
ALL_MODULES       := $(SHARE_MODULES) $(ALL_SERVICES)

# ── Proto ───────────────────────────────────────────────────
PROTOC_GEN_GO_VER      ?= v1.36.11
PROTOC_GEN_GO_GRPC_VER ?= v1.6.1

ifeq ($(GOHOSTOS), windows)
	API_PROTO_FILES=$(shell git bash -c "find api -name '*.proto'")
else
	API_PROTO_FILES=$(shell find api -name '*.proto')
endif

.PHONY: init api proto test build lint smoke smoke-setup smoke-func migrate docker-up docker-down \
        activity stock risk order support gateway processor job \
        bin docker-bin docker-build

# ── 工具安装 ─────────────────────────────────────────────────
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VER)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VER)
	which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ── Proto 生成 ──────────────────────────────────────────────
proto api:
	PATH="$(GOPATH)/bin:$(PATH)" $(PROTOC) --proto_path=. \
		--go_out=paths=source_relative:. \
		--go-grpc_out=paths=source_relative:. \
		$(API_PROTO_FILES)

# ── 测试 ────────────────────────────────────────────────────
test:
	@for mod in $(ALL_MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && go test -count=1 ./...); \
	done

# ── 编译 ────────────────────────────────────────────────────
build:
	@for mod in $(ALL_MODULES); do \
		echo "==> $$mod"; \
		(cd $$mod && go mod tidy && go build ./...); \
	done

# ── Lint ─────────────────────────────────────────────────────
lint:
	golangci-lint run ./...

# ── 端到端烟测 ───────────────────────────────────────────────
# ── 数据库 Migrations ─────────────────────────────────────────
migrate:
	./scripts/run-migrations.sh

smoke-setup: migrate
	./scripts/smoke-setup.sh

smoke:
	./scripts/smoke.sh

smoke-func:
	SMOKE_MODE=func ./scripts/smoke.sh

# ── 单服务运行（本地开发）──────────────────────────────────────
activity: ; cd services/activity-service  && go run ./cmd
stock:    ; cd services/stock-service     && go run ./cmd
risk:     ; cd services/risk-service      && go run ./cmd
order:    ; cd services/order-service     && go run ./cmd
support:  ; cd services/support-service   && go run ./cmd
gateway:  ; cd services/seckill-gateway   && go run ./cmd
processor:; cd services/seckill-processor && go run ./cmd
job:      ; cd services/seckill-job       && go run ./cmd

# ── Docker ───────────────────────────────────────────────────
docker-up:
	./scripts/docker-build.sh
	docker compose up -d --scale seckill-processor=3

docker-down:
	docker compose down

redis postgres:
	docker compose up -d $@

# ── 本地编译为 Linux 二进制 ────────────────────────────────────
BUILDDIR := build
LDFLAGS  := -s -w
SERVICES := activity-service stock-service risk-service order-service \
            support-service seckill-gateway seckill-processor seckill-job

bin:
	@mkdir -p $(BUILDDIR)
	@for svc in $(SERVICES); do \
		echo "==> $$svc"; \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
			go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$$svc ./services/$$svc/cmd 2>&1; \
	done

docker-build:
	./scripts/docker-build.sh
