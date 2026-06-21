package rpcclient

import (
	"fmt"
	"strconv"

	"seckill-common/config"
)

// CircuitBreakerPolicyFromMap 从配置映射中读取 gRPC client 熔断策略。
func CircuitBreakerPolicyFromMap(raw map[string]interface{}, path string) CircuitBreakerPolicy {
	return CircuitBreakerPolicy{
		Success: floatFromMap(raw, path+".success"),
		Request: int64(config.GetInt(raw, path+".request")),
		Window:  config.GetDuration(raw, path+".window", 0),
		Bucket:  config.GetInt(raw, path+".bucket"),
	}
}

func floatFromMap(raw map[string]interface{}, path string) float64 {
	value, ok := config.Lookup(raw, path)
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return parsed
		}
		return 0
	default:
		parsed, err := strconv.ParseFloat(fmt.Sprint(v), 64)
		if err == nil {
			return parsed
		}
		return 0
	}
}
