-- wrk-part-in.lua — 秒杀压测脚本
-- 从环境变量读取参数，避免硬编码。用随机 userId 绕过单用户限流。

wrk.method = "POST"
wrk.path = "/api/seckill/part-in"

-- 环境变量读取
local run_id = os.getenv("SMOKE_RUN_ID") or ""
local activity_no = os.getenv("SMOKE_ACTIVITY_NO") or "1001"
local sku_no = os.getenv("SMOKE_SKU_NO") or "SKU001"
local base_user = tonumber(os.getenv("SMOKE_BASE_USER")) or 100000
local user_count = tonumber(os.getenv("SMOKE_USER_COUNT")) or 100

-- 预生成 body（固定部分）
local body_fmt = string.format(
    '{"activityNo":"%s","skuNo":"%s","requestIP":"127.0.0.1"}',
    activity_no, sku_no
)

counter = 0

function request()
    counter = counter + 1
    local user_id = base_user + (counter % user_count)

    local headers = {}
    headers["Content-Type"] = "application/json"
    headers["X-User-Id"] = tostring(user_id)
    headers["X-Smoke-Run-Id"] = run_id

    return wrk.format(nil, wrk.path, headers, body_fmt)
end
