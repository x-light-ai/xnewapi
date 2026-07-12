// FORK-CUSTOM: Keep SuccessRateSelector retry policy outside the upstream relay controller.
package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func ShouldContinueRelayRetry(param *RetryParam) bool {
	return operation_setting.GetSuccessRateSelectorEnabled() || param.GetRetry() <= common.RetryTimes
}

func ShouldRetryAfterStatusError(retryTimes int) bool {
	return operation_setting.GetSuccessRateSelectorEnabled() || retryTimes > 0
}
