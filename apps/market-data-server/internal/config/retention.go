package config

// RetentionPolicy 数据保留策略
//
// 定义不同周期的 K 线应该保留多少根历史数据
type RetentionPolicy struct {
	CandleHistoryLength map[string]int // bar -> 保留数量
}

// DefaultRetentionPolicy 默认保留策略
//
// 保留策略说明：
// - 1s: 60根（1分钟）
// - 1m: 200根（3.3小时）
// - 5m: 200根（16.6小时）
// - 1H: 200根（8.3天）
// - 1D: 365根（1年）
func DefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		CandleHistoryLength: map[string]int{
			"1s":  60,  // 1分钟
			"1m":  200, // 3.3小时
			"3m":  200, // 10小时
			"5m":  200, // 16.6小时
			"15m": 200, // 2.08天
			"30m": 200, // 4.16天
			"1H":  200, // 8.3天
			"2H":  200, // 16.6天
			"4H":  200, // 33.3天
			"1D":  365, // 1年
			"1W":  104, // 2年
			"1M":  60,  // 5年
		},
	}
}

// GetMaxLength 获取指定周期的最大保留数量
func (p *RetentionPolicy) GetMaxLength(bar string) int {
	if length, ok := p.CandleHistoryLength[bar]; ok {
		return length
	}
	return 100 // 默认保留 100 根
}
