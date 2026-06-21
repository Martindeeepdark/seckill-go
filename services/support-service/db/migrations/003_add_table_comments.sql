-- +migrate Up

-- 用户表注释
COMMENT ON TABLE t_user IS '用户信息表';

COMMENT ON COLUMN t_user.id IS '用户主键 ID（BIGSERIAL）';
COMMENT ON COLUMN t_user.username IS '用户名（唯一，50 字符）';
COMMENT ON COLUMN t_user.phone IS '手机号（唯一，20 字符）';
COMMENT ON COLUMN t_user.nickname IS '昵称（50 字符）';
COMMENT ON COLUMN t_user.member_level IS '会员等级：0（普通用户）、1（VIP1）、2（VIP2）等';
COMMENT ON COLUMN t_user.status IS '用户状态：0（正常）、1（冻结）';
COMMENT ON COLUMN t_user.created_at IS '创建时间';
COMMENT ON COLUMN t_user.updated_at IS '更新时间';

-- 支付表注释
COMMENT ON TABLE t_payment IS '支付信息表';

COMMENT ON COLUMN t_payment.id IS '支付主键 ID（BIGSERIAL）';
COMMENT ON COLUMN t_payment.payment_no IS '支付单号（唯一，32 字符）';
COMMENT ON COLUMN t_payment.order_no IS '关联订单号';
COMMENT ON COLUMN t_payment.user_id IS '用户 ID';
COMMENT ON COLUMN t_payment.pay_amount IS '支付金额（单位：分）';
COMMENT ON COLUMN t_payment.pay_channel IS '支付渠道：MOCK（模拟支付）、ALIPAY（支付宝）、WECHAT（微信）等';
COMMENT ON COLUMN t_payment.pay_status IS '支付状态：0（待支付）、1（已支付）、2（已取消）、3（已退款）';
COMMENT ON COLUMN t_payment.transaction_no IS '第三方支付平台交易号';
COMMENT ON COLUMN t_payment.paid_at IS '支付完成时间';
COMMENT ON COLUMN t_payment.created_at IS '创建时间';
COMMENT ON COLUMN t_payment.updated_at IS '更新时间';

-- 免单卡表注释
COMMENT ON TABLE t_free_card IS '免单卡信息表';

COMMENT ON COLUMN t_free_card.id IS '免单卡主键 ID（BIGSERIAL）';
COMMENT ON COLUMN t_free_card.card_no IS '免单卡卡号（唯一，32 字符）';
COMMENT ON COLUMN t_free_card.card_name IS '免单卡名称';
COMMENT ON COLUMN t_free_card.face_value IS '面值（单位：分）';
COMMENT ON COLUMN t_free_card.user_id IS '绑定用户 ID（NULL 表示未绑定）';
COMMENT ON COLUMN t_free_card.order_no IS '使用的订单号（NULL 表示未使用）';
COMMENT ON COLUMN t_free_card.status IS '卡状态：0（未激活）、1（已激活）、2（已使用）、3（已过期）';
COMMENT ON COLUMN t_free_card.valid_days IS '有效天数（默认 365 天）';
COMMENT ON COLUMN t_free_card.activated_at IS '激活时间';
COMMENT ON COLUMN t_free_card.expire_at IS '过期时间';
COMMENT ON COLUMN t_free_card.created_at IS '创建时间';
COMMENT ON COLUMN t_free_card.updated_at IS '更新时间';

-- +migrate Down

COMMENT ON TABLE t_user IS NULL;
COMMENT ON COLUMN t_user.id IS NULL;
COMMENT ON COLUMN t_user.username IS NULL;
COMMENT ON COLUMN t_user.phone IS NULL;
COMMENT ON COLUMN t_user.nickname IS NULL;
COMMENT ON COLUMN t_user.member_level IS NULL;
COMMENT ON COLUMN t_user.status IS NULL;
COMMENT ON COLUMN t_user.created_at IS NULL;
COMMENT ON COLUMN t_user.updated_at IS NULL;

COMMENT ON TABLE t_payment IS NULL;
COMMENT ON COLUMN t_payment.id IS NULL;
COMMENT ON COLUMN t_payment.payment_no IS NULL;
COMMENT ON COLUMN t_payment.order_no IS NULL;
COMMENT ON COLUMN t_payment.user_id IS NULL;
COMMENT ON COLUMN t_payment.pay_amount IS NULL;
COMMENT ON COLUMN t_payment.pay_channel IS NULL;
COMMENT ON COLUMN t_payment.pay_status IS NULL;
COMMENT ON COLUMN t_payment.transaction_no IS NULL;
COMMENT ON COLUMN t_payment.paid_at IS NULL;
COMMENT ON COLUMN t_payment.created_at IS NULL;
COMMENT ON COLUMN t_payment.updated_at IS NULL;

COMMENT ON TABLE t_free_card IS NULL;
COMMENT ON COLUMN t_free_card.id IS NULL;
COMMENT ON COLUMN t_free_card.card_no IS NULL;
COMMENT ON COLUMN t_free_card.card_name IS NULL;
COMMENT ON COLUMN t_free_card.face_value IS NULL;
COMMENT ON COLUMN t_free_card.user_id IS NULL;
COMMENT ON COLUMN t_free_card.order_no IS NULL;
COMMENT ON COLUMN t_free_card.status IS NULL;
COMMENT ON COLUMN t_free_card.valid_days IS NULL;
COMMENT ON COLUMN t_free_card.activated_at IS NULL;
COMMENT ON COLUMN t_free_card.expire_at IS NULL;
COMMENT ON COLUMN t_free_card.created_at IS NULL;
COMMENT ON COLUMN t_free_card.updated_at IS NULL;
