// FORK-CUSTOM: Keep SuccessRateSelector retry policy outside the upstream relay controller.
package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func ShouldContinueRelayRetry(param *RetryParam) bool {
	if operation_setting.GetSuccessRateSelectorEnabled() {
		// SuccessRateSelector 会在所有优先级层级中选择渠道,每次重试排除已失败的渠道
		// 一般配置: 2-3 个优先级层级 × 每层 5-10 个渠道 = 10-30 个渠道
		// 考虑到熔断恢复、半开探测等机制可能导致同一渠道被重试,设置上限为 50 次
		// 既能覆盖大部分实际场景,又能防止异常情况下的无限重试
		return param.GetRetry() <= 50
	}
	return param.GetRetry() <= common.RetryTimes
}

func ShouldRetryAfterStatusError(retryTimes int) bool {
	return operation_setting.GetSuccessRateSelectorEnabled() || retryTimes > 0
}
