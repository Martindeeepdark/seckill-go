-- +migrate Up

-- 风控记录表注释
COMMENT ON TABLE sk_risk_record IS '风控记录表（记录用户行为风险评估）';

COMMENT ON COLUMN sk_risk_record.id IS '风控记录主键 ID（BIGSERIAL）';
COMMENT ON COLUMN sk_risk_record.user_id IS '用户 ID';
COMMENT ON COLUMN sk_risk_record.action_type IS '行为类型：SECKILL（秒杀）、LOGIN（登录）、REGISTER（注册）等';
COMMENT ON COLUMN sk_risk_record.risk_level IS '风险等级：0（正常）、1（低风险）、2（中风险）、3（高风险）';
COMMENT ON COLUMN sk_risk_record.request_ip IS '请求来源 IP 地址';
COMMENT ON COLUMN sk_risk_record.request_info IS '请求详细信息（JSON 格式）';
COMMENT ON COLUMN sk_risk_record.created_at IS '创建时间';

-- +migrate Down

COMMENT ON TABLE sk_risk_record IS NULL;
COMMENT ON COLUMN sk_risk_record.id IS NULL;
COMMENT ON COLUMN sk_risk_record.user_id IS NULL;
COMMENT ON COLUMN sk_risk_record.action_type IS NULL;
COMMENT ON COLUMN sk_risk_record.risk_level IS NULL;
COMMENT ON COLUMN sk_risk_record.request_ip IS NULL;
COMMENT ON COLUMN sk_risk_record.request_info IS NULL;
COMMENT ON COLUMN sk_risk_record.created_at IS NULL;
