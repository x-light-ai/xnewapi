package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	defaultChannelSuccessRateHalfLifeSeconds          = 1800
	defaultChannelSuccessRateExploreRate              = 0.02
	defaultChannelSuccessRateConsecutiveFailThreshold = 3
	defaultChannelSuccessRateCircuitScope             = "model"
	defaultChannelSuccessRateHalfOpenSuccessThreshold = 2
)

type ChannelSuccessRateImmediateDisableSetting struct {
	Enabled     bool     `json:"enabled"`
	StatusCodes []int    `json:"status_codes"`
	ErrorCodes  []string `json:"error_codes"`
	ErrorTypes  []string `json:"error_types"`
}

type ChannelSuccessRateHealthManagerSetting struct {
	CircuitScope             string  `json:"circuit_scope"`
	DisableThreshold         float64 `json:"disable_threshold"`
	EnableThreshold          float64 `json:"enable_threshold"`
	MinSampleSize            int     `json:"min_sample_size"`
	RecoveryCheckInterval    int     `json:"recovery_check_interval"`
	HalfOpenSuccessThreshold int     `json:"half_open_success_threshold"`
}

type ChannelSuccessRateSetting struct {
	Enabled                  bool                                    `json:"enabled"`
	HalfLifeSeconds          int                                     `json:"half_life_seconds"`
	ExploreRate              float64                                 `json:"explore_rate"`
	QuickDowngrade           bool                                    `json:"quick_downgrade"`
	ConsecutiveFailThreshold int                                     `json:"consecutive_fail_threshold"`
	PriorityWeights          map[int]float64                         `json:"priority_weights"`
	ImmediateDisable         ChannelSuccessRateImmediateDisableSetting `json:"immediate_disable"`
	HealthManager            ChannelSuccessRateHealthManagerSetting    `json:"health_manager"`
}

var channelSuccessRateSetting = ChannelSuccessRateSetting{
	Enabled:                  false,
	HalfLifeSeconds:          defaultChannelSuccessRateHalfLifeSeconds,
	ExploreRate:              defaultChannelSuccessRateExploreRate,
	QuickDowngrade:           true,
	ConsecutiveFailThreshold: defaultChannelSuccessRateConsecutiveFailThreshold,
	PriorityWeights: map[int]float64{
		10: 0.2,
		5:  0,
		0:  -0.1,
	},
	ImmediateDisable: ChannelSuccessRateImmediateDisableSetting{
		Enabled:     true,
		StatusCodes: []int{400, 401, 403, 404, 500, 502},
	},
	HealthManager: ChannelSuccessRateHealthManagerSetting{
		CircuitScope:             defaultChannelSuccessRateCircuitScope,
		DisableThreshold:         0.2,
		EnableThreshold:          0.7,
		MinSampleSize:            10,
		RecoveryCheckInterval:    600,
		HalfOpenSuccessThreshold: defaultChannelSuccessRateHalfOpenSuccessThreshold,
	},
}

func init() {
	config.GlobalConfig.Register("channel_success_rate_setting", &channelSuccessRateSetting)
}

func GetChannelSuccessRateSetting() *ChannelSuccessRateSetting {
	return &channelSuccessRateSetting
}

func GetSuccessRateSelectorEnabled() bool {
	return channelSuccessRateSetting.Enabled
}
