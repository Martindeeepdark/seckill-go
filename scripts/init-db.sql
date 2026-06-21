-- 秒杀基础域（seckill-base）：活动、商品、订单
CREATE DATABASE seckill_activity;
CREATE DATABASE seckill_order;

-- 支撑域（seckill-support）：用户、支付、卡片、风控
CREATE DATABASE seckill_risk;
CREATE DATABASE seckill_support;

-- 授权
GRANT ALL PRIVILEGES ON DATABASE seckill_activity TO seckill;
GRANT ALL PRIVILEGES ON DATABASE seckill_order TO seckill;
GRANT ALL PRIVILEGES ON DATABASE seckill_risk TO seckill;
GRANT ALL PRIVILEGES ON DATABASE seckill_support TO seckill;
