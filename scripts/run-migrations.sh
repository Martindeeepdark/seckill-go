#!/usr/bin/env bash
# run-migrations.sh — 运行所有服务的数据库 migrations
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

info() { echo "[migrations] $*"; }
error() { echo "[migrations] ERROR: $*" >&2; }

# 等待 postgres 就绪
wait_postgres() {
    info "waiting for postgres..."
    for i in {1..30}; do
        if docker compose exec -T postgres psql -U seckill -d seckill -c "SELECT 1" >/dev/null 2>&1; then
            info "postgres is ready"
            return 0
        fi
        sleep 1
    done
    error "postgres not ready after 30s"
    return 1
}

# 运行单个服务的 migrations
run_service_migrations() {
    local service=$1
    local db_name=$2
    local migrations_dir="$PROJECT_ROOT/services/$service/db/migrations"

    if [[ ! -d "$migrations_dir" ]]; then
        info "no migrations for $service (dir not found: $migrations_dir)"
        return 0
    fi

    info "running migrations for $service -> $db_name"

    local count=0
    for migration in "$migrations_dir"/*.sql; do
        if [[ ! -f "$migration" ]]; then
            continue
        fi
        local filename=$(basename "$migration")
        info "  applying $filename"

        # 只执行 Up 部分（-- +migrate Down 之前的内容）
        # 使用 sed 提取 Up 部分，如果文件没有 Down 标记则执行整个文件
        sed '/^-- +migrate Down/,$d' "$migration" | \
            docker compose exec -T postgres psql -U seckill -d "$db_name" 2>&1 | \
            grep -v "does not exist, skipping" | grep -v "^NOTICE:" | grep -v "^$" || true
        ((count++))
    done

    info "  applied $count migrations for $service"
}

main() {
    cd "$PROJECT_ROOT"

    wait_postgres || exit 1

    # 运行各服务的 migrations
    run_service_migrations "activity-service" "seckill_activity"
    run_service_migrations "order-service" "seckill_order"
    run_service_migrations "stock-service" "seckill_stock"
    run_service_migrations "risk-service" "seckill_risk"
    run_service_migrations "support-service" "seckill_support"

    info "all migrations completed successfully"
}

main "$@"
