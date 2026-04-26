package service

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type channelSuccessRateState struct {
	success                  float64
	failure                  float64
	updated                  time.Time
	consecutiveFails         int
	observed                 int
	temporaryOpenUntil       time.Time
	temporaryOpenReason      string
	halfOpenUntil            time.Time
	halfOpenReason           string
	halfOpenSuccesses        int
	halfOpenSuccessThreshold int
	halfOpenProbeInFlight    bool
}

type channelFailureEvent struct {
	Group            string
	ModelName        string
	ChannelID        int
	Reason           string
	ErrorCode        string
	ErrorType        string
	StatusCode       int
	ErrorMsg         string
	Score            float64
	Observed         int
	ConsecutiveFails int
	CircuitScope     string
}

type ChannelSuccessRateRuntimeState struct {
	TemporaryCircuitOpen   bool
	TemporaryCircuitUntil  time.Time
	TemporaryCircuitReason string
	CurrentWeightedScore   float64
	Observed               int
	Group                  string
	ModelName              string
	HalfOpen               bool
	HalfOpenUntil          time.Time
	HalfOpenReason         string
	HalfOpenSuccesses      int
	CircuitScope           string
}

type successRateRNG interface {
	Float64() float64
	Intn(n int) int
}

type defaultSuccessRateRNG struct{}

func (defaultSuccessRateRNG) Float64() float64 { return rand.Float64() }
func (defaultSuccessRateRNG) Intn(n int) int   { return rand.Intn(n) }

type channelFailureCallback func(event channelFailureEvent)

type channelSuccessRateSelector struct {
	mu sync.Mutex

	maxKeys          int
	now              func() time.Time
	rng              successRateRNG
	state            map[string]channelSuccessRateState
	circuitState     map[string]channelSuccessRateState
	rrCursors        map[string]int
	failureCallbacks []channelFailureCallback
	scoreOverrides   map[int]float64
}

var defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()

func ResetChannelSuccessRateSelectorForTest() {
	defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
}

func ResetChannelSuccessRateHealthManagerForTest() {
	channelSuccessRateHealthManagerStartOnce = sync.Once{}
	defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
}

func SetChannelSuccessRateNowForTest(now func() time.Time) {
	if now == nil {
		defaultChannelSuccessRateSelector.now = time.Now
		return
	}
	defaultChannelSuccessRateSelector.now = now
}

func ClearExpiredChannelSuccessRateCircuitsForTest() {
	defaultChannelSuccessRateSelector.clearExpiredTemporaryCircuits(buildChannelSuccessRateConfig())
}

func newChannelSuccessRateSelector() *channelSuccessRateSelector {
	return &channelSuccessRateSelector{
		maxKeys:          4096,
		now:              time.Now,
		rng:              defaultSuccessRateRNG{},
		state:            make(map[string]channelSuccessRateState),
		circuitState:     make(map[string]channelSuccessRateState),
		rrCursors:        make(map[string]int),
		failureCallbacks: make([]channelFailureCallback, 0),
		scoreOverrides:   make(map[int]float64),
	}
}

func getUsedChannelIDs(ctx *gin.Context) map[int]bool {
	if ctx == nil {
		return nil
	}
	used := ctx.GetStringSlice("use_channel")
	if len(used) == 0 {
		return nil
	}
	ids := make(map[int]bool, len(used))
	for _, s := range used {
		var id int
		if _, err := fmt.Sscanf(s, "%d", &id); err == nil && id > 0 {
			ids[id] = true
		}
	}
	return ids
}

type selectionDecision struct {
	channelID int
	name      string
	priority  int64
	score     float64
	reason    string
}

func formatSelectionDecision(decision selectionDecision) string {
	if decision.reason != "" {
		return fmt.Sprintf("#%d(%s，权重%d)：%s", decision.channelID, decision.name, decision.priority, decision.reason)
	}
	return fmt.Sprintf("#%d(%s，权重%d)：当前权重分数%.4f", decision.channelID, decision.name, decision.priority, decision.score)
}

func selectChannelWithSuccessRate(ctx *gin.Context, group string, modelName string, retry int) (*model.Channel, error) {
	if !operation_setting.GetSuccessRateSelectorEnabled() {
		return model.GetRandomSatisfiedChannel(group, modelName, retry)
	}
	stageCount := model.GetPriorityStageCount(group, modelName)
	if stageCount == 0 {
		return nil, nil
	}
	cfg := buildChannelSuccessRateConfig()
	for stage := 0; stage < stageCount; stage++ {
		channels, err := model.GetSatisfiedChannels(group, modelName, stage)
		if err != nil {
			return nil, err
		}
		if len(channels) == 0 {
			continue
		}
		excludeIDs := getUsedChannelIDs(ctx)
		available := channels
		if len(excludeIDs) > 0 {
			filtered := make([]*model.Channel, 0, len(channels))
			for _, ch := range channels {
				if ch != nil && !excludeIDs[ch.Id] {
					filtered = append(filtered, ch)
				}
			}
			available = filtered
		}
		if len(available) == 0 {
			continue
		}
		selected, selectedScore, others := defaultChannelSuccessRateSelector.pickDetailed(group, modelName, available, cfg)
		if selected == nil {
			if len(others) > 0 {
				appendChannelSelectionLog(defaultChannelSuccessRateSelector.now(), modelName, group, len(available), 0, "", 0, 0, others)
			}
			continue
		}
		appendChannelSelectionLog(defaultChannelSuccessRateSelector.now(), modelName, group, len(available), selected.Id, selected.Name, selected.GetPriority(), selectedScore, others)
		return selected, nil
	}
	return nil, nil
}

func SelectBySuccessRate(ctx *gin.Context, group string, modelName string, retry int) (*model.Channel, error) {
	channels, err := model.GetSatisfiedChannels(group, modelName, retry)
	if err != nil || len(channels) == 0 {
		return nil, err
	}
	excludeIDs := getUsedChannelIDs(ctx)
	if len(excludeIDs) > 0 {
		filtered := channels[:0]
		for _, ch := range channels {
			if !excludeIDs[ch.Id] {
				filtered = append(filtered, ch)
			}
		}
		channels = filtered
	}
	cfg := buildChannelSuccessRateConfig()
	selected, selectedScore, others := defaultChannelSuccessRateSelector.pickDetailed(group, modelName, channels, cfg)
	if selected != nil {
		appendChannelSelectionLog(defaultChannelSuccessRateSelector.now(), modelName, group, len(channels), selected.Id, selected.Name, selected.GetPriority(), selectedScore, others)
	} else if len(others) > 0 {
		appendChannelSelectionLog(defaultChannelSuccessRateSelector.now(), modelName, group, len(channels), 0, "", 0, 0, others)
	}
	return selected, nil
}

func ObserveChannelRequestResult(c *gin.Context, success bool, err *types.NewAPIError) {
	if c == nil {
		return
	}
	channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	if channelID <= 0 {
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	}
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	latency := getChannelRequestLatency(c)
	cfg := buildChannelSuccessRateConfig()

	model.ObserveChannelRuntime(group, modelName, channelID, success, latency)
	defaultChannelSuccessRateSelector.observeDetailed(group, modelName, channelID, success, err, cfg)
	if !success {
		appendChannelRequestObservationLog(defaultChannelSuccessRateSelector.now(), group, modelName, channelID, err, cfg)
	}
}

func appendChannelRequestObservationLog(now time.Time, group string, modelName string, channelID int, err *types.NewAPIError, cfg channelSuccessRateConfig) {
	channelName := ""
	priority := int64(0)
	channel, channelErr := model.CacheGetChannel(channelID)
	if channelErr == nil && channel != nil {
		channelName = channel.Name
		priority = channel.GetPriority()
	}
	score := defaultChannelSuccessRateSelector.getScore(group, modelName, channelID, cfg)
	detail := "请求失败"
	if err != nil {
		detail = fmt.Sprintf("请求失败：status=%d code=%s", err.StatusCode, err.GetErrorCode())
	}
	circuitReason := ""
	state := defaultChannelSuccessRateSelector.GetRuntimeStateForChannel(channelID, cfg)
	if state.TemporaryCircuitOpen {
		circuitReason = state.TemporaryCircuitReason
	}
	appendChannelSelectionObservationLog(now, modelName, group, channelID, channelName, priority, score, false, detail, circuitReason)
}

func GetChannelSuccessRateScore(group string, modelName string, channelID int) float64 {
	return defaultChannelSuccessRateSelector.getScore(group, modelName, channelID, buildChannelSuccessRateConfig())
}

func GetChannelSuccessRateSampleSize(group string, modelName string, channelID int) int {
	return defaultChannelSuccessRateSelector.getSampleSize(group, modelName, channelID)
}

func GetChannelSuccessRateRuntimeState(channelID int) ChannelSuccessRateRuntimeState {
	return defaultChannelSuccessRateSelector.GetRuntimeStateForChannel(channelID, buildChannelSuccessRateConfig())
}

func getChannelRequestLatency(c *gin.Context) time.Duration {
	if c == nil {
		return 0
	}
	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		return 0
	}
	latency := time.Since(startTime)
	if latency < 0 {
		return 0
	}
	return latency
}

func (s *channelSuccessRateSelector) pick(group string, modelName string, channels []*model.Channel, cfg channelSuccessRateConfig) *model.Channel {
	selected, _, _ := s.pickDetailed(group, modelName, channels, cfg)
	return selected
}

func (s *channelSuccessRateSelector) pickDetailed(group string, modelName string, channels []*model.Channel, cfg channelSuccessRateConfig) (*model.Channel, float64, []selectionDecision) {
	if s == nil || len(channels) == 0 {
		return nil, 0, nil
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	if len(channels) == 1 {
		state, _, recovered, halfOpenBlocked := s.getSelectionState(group, modelName, channels[0].Id, now, cfg)
		if recovered {
			recordTemporaryCircuitRecovered(s.circuitKeyForScope(successRateGroupKey(group), modelName, channels[0].Id, cfg.CircuitScope), now)
		}
		if isTemporaryCircuitOpenAt(state, now) {
			return nil, 0, []selectionDecision{{
				channelID: channels[0].Id,
				name:      channels[0].Name,
				priority:  channels[0].GetPriority(),
				reason:    temporaryCircuitReasonAt(state, now),
			}}
		}
		if halfOpenBlocked {
			return nil, 0, []selectionDecision{{
				channelID: channels[0].Id,
				name:      channels[0].Name,
				priority:  channels[0].GetPriority(),
				reason:    halfOpenReasonAt(state, now),
			}}
		}
		if isHalfOpenAt(state, now) {
			circuitKey := s.circuitKeyForScope(successRateGroupKey(group), modelName, channels[0].Id, cfg.CircuitScope)
			s.mu.Lock()
			circuitState := s.circuitState[circuitKey]
			if isHalfOpenAt(circuitState, now) && !circuitState.halfOpenProbeInFlight {
				circuitState.halfOpenProbeInFlight = true
				s.circuitState[circuitKey] = circuitState
			}
			s.mu.Unlock()
		}
		return channels[0], s.getScore(group, modelName, channels[0].Id, cfg), nil
	}
	if cfg.ExploreRate > 0 && s.rng != nil && s.rng.Float64() < cfg.ExploreRate {
		available := make([]*model.Channel, 0, len(channels))
		others := make([]selectionDecision, 0, len(channels)-1)
		for _, channel := range channels {
			if channel == nil {
				continue
			}
			state, _, recovered, halfOpenBlocked := s.getSelectionState(group, modelName, channel.Id, now, cfg)
			if recovered {
				recordTemporaryCircuitRecovered(s.circuitKeyForScope(successRateGroupKey(group), modelName, channel.Id, cfg.CircuitScope), now)
			}
			if isTemporaryCircuitOpenAt(state, now) {
				others = append(others, selectionDecision{
					channelID: channel.Id,
					name:      channel.Name,
					priority:  channel.GetPriority(),
					reason:    temporaryCircuitReasonAt(state, now),
				})
				continue
			}
			if halfOpenBlocked {
				others = append(others, selectionDecision{
					channelID: channel.Id,
					name:      channel.Name,
					priority:  channel.GetPriority(),
					reason:    halfOpenReasonAt(state, now),
				})
				continue
			}
			available = append(available, channel)
		}
		if len(available) > 0 {
			selected := available[0]
			if len(available) > 1 {
				selected = available[s.rng.Intn(len(available))]
			}
			selectedScore := s.getScore(group, modelName, selected.Id, cfg)
			for _, other := range available {
				if other.Id == selected.Id {
					continue
				}
				others = append(others, s.buildSelectionDecision(group, modelName, other, cfg, now))
			}
			return selected, selectedScore, others
		}
	}

	type scoredChannel struct {
		channel *model.Channel
		score   float64
	}
	keyGroup := successRateGroupKey(group)
	scored := make([]scoredChannel, 0, len(channels))
	others := make([]selectionDecision, 0, len(channels))

	s.mu.Lock()
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		state, _, recovered, halfOpenBlocked := s.getSelectionStateLocked(group, modelName, channel.Id, now, cfg)
		if recovered {
			recordTemporaryCircuitRecovered(s.circuitKeyForScope(keyGroup, modelName, channel.Id, cfg.CircuitScope), now)
		}
		if isTemporaryCircuitOpenAt(state, now) {
			others = append(others, selectionDecision{
				channelID: channel.Id,
				name:      channel.Name,
				priority:  channel.GetPriority(),
				reason:    temporaryCircuitReasonAt(state, now),
			})
			continue
		}
		if halfOpenBlocked {
			others = append(others, selectionDecision{
				channelID: channel.Id,
				name:      channel.Name,
				priority:  channel.GetPriority(),
				reason:    halfOpenReasonAt(state, now),
			})
			continue
		}
		scoreState := s.state[s.scoreKey(keyGroup, successRateModelKey(modelName), channel.Id)]
		scored = append(scored, scoredChannel{
			channel: channel,
			score:   scoreForStateLocked(scoreState, channel.GetPriority(), cfg),
		})
	}
	if len(scored) == 0 {
		s.mu.Unlock()
		return nil, 0, others
	}
	best := scored[0].score
	for i := 1; i < len(scored); i++ {
		if scored[i].score > best {
			best = scored[i].score
		}
	}
	const eps = 1e-12
	ties := make([]*model.Channel, 0, len(scored))
	for _, item := range scored {
		if math.Abs(item.score-best) <= eps {
			ties = append(ties, item.channel)
		} else {
			others = append(others, selectionDecision{
				channelID: item.channel.Id,
				name:      item.channel.Name,
				priority:  item.channel.GetPriority(),
				score:     item.score,
				reason:    fmt.Sprintf("当前权重分数%.4f，低于最佳%.4f", item.score, best),
			})
		}
	}
	selected := ties[0]
	if len(ties) > 1 {
		rrKey := keyGroup + ":" + successRateModelKey(modelName)
		idx := s.ensureRRCursorLocked(rrKey)
		s.rrCursors[rrKey] = idx + 1
		selected = ties[idx%len(ties)]
		for _, item := range ties {
			if item.Id == selected.Id {
				continue
			}
			state := s.state[s.scoreKey(keyGroup, successRateModelKey(modelName), item.Id)]
			others = append(others, selectionDecision{
				channelID: item.Id,
				name:      item.Name,
				priority:  item.GetPriority(),
				score:     scoreForStateLocked(state, item.GetPriority(), cfg),
				reason:    "当前权重分数并列，轮询让给其它同分渠道",
			})
		}
	}
	selectedState := s.state[s.scoreKey(keyGroup, successRateModelKey(modelName), selected.Id)]
	selectedCircuitKey := s.circuitKeyForScope(keyGroup, modelName, selected.Id, cfg.CircuitScope)
	selectedCircuitState := s.circuitState[selectedCircuitKey]
	if isHalfOpenAt(selectedCircuitState, now) && !selectedCircuitState.halfOpenProbeInFlight {
		selectedCircuitState.halfOpenProbeInFlight = true
		s.circuitState[selectedCircuitKey] = selectedCircuitState
	}
	selectedScore := scoreForStateLocked(selectedState, selected.GetPriority(), cfg)
	s.mu.Unlock()
	return selected, selectedScore, others
}

func isTemporaryCircuitOpenAt(state channelSuccessRateState, now time.Time) bool {
	return !state.temporaryOpenUntil.IsZero() && now.Before(state.temporaryOpenUntil)
}

func isHalfOpenAt(state channelSuccessRateState, now time.Time) bool {
	return !state.halfOpenUntil.IsZero() && now.Before(state.halfOpenUntil)
}

func canHalfOpenProbe(state channelSuccessRateState, now time.Time) bool {
	return isHalfOpenAt(state, now) && !state.halfOpenProbeInFlight
}

func (s *channelSuccessRateSelector) getSelectionState(group string, modelName string, channelID int, now time.Time, cfg channelSuccessRateConfig) (channelSuccessRateState, string, bool, bool) {
	if s == nil || channelID <= 0 {
		return channelSuccessRateState{}, "", false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getSelectionStateLocked(group, modelName, channelID, now, cfg)
}

func (s *channelSuccessRateSelector) getSelectionStateLocked(group string, modelName string, channelID int, now time.Time, cfg channelSuccessRateConfig) (channelSuccessRateState, string, bool, bool) {
	circuitKey := s.circuitKeyForScope(successRateGroupKey(group), modelName, channelID, cfg.CircuitScope)
	state, recovered := s.decayLockedForKey(circuitKey, s.circuitState[circuitKey], now, cfg)
	s.circuitState[circuitKey] = state
	return state, circuitKey, recovered, isHalfOpenAt(state, now) && !canHalfOpenProbe(state, now)
}

func temporaryCircuitReasonAt(state channelSuccessRateState, now time.Time) string {
	if !isTemporaryCircuitOpenAt(state, now) {
		return ""
	}
	if state.temporaryOpenReason == "" {
		return fmt.Sprintf("临时熔断至 %s", state.temporaryOpenUntil.Format("2006-01-02 15:04:05"))
	}
	return fmt.Sprintf("%s，熔断至 %s", state.temporaryOpenReason, state.temporaryOpenUntil.Format("2006-01-02 15:04:05"))
}

func halfOpenReasonAt(state channelSuccessRateState, now time.Time) string {
	if !isHalfOpenAt(state, now) {
		return ""
	}
	required := state.halfOpenSuccessThreshold
	if required <= 0 {
		required = 1
	}
	if state.halfOpenReason == "" {
		return fmt.Sprintf("半开探测中（%d/%d）至 %s", state.halfOpenSuccesses, required, state.halfOpenUntil.Format("2006-01-02 15:04:05"))
	}
	return fmt.Sprintf("%s（%d/%d）至 %s", state.halfOpenReason, state.halfOpenSuccesses, required, state.halfOpenUntil.Format("2006-01-02 15:04:05"))
}

func scoreForStateLocked(state channelSuccessRateState, priority int64, cfg channelSuccessRateConfig) float64 {
	score := (state.success + 1) / (state.success + state.failure + 2)
	if cfg.ConsecutiveFailThreshold > 0 && state.consecutiveFails >= cfg.ConsecutiveFailThreshold {
		penaltySteps := state.consecutiveFails - cfg.ConsecutiveFailThreshold + 1
		score /= float64(1 + penaltySteps*4)
	}
	if cfg.PriorityWeights != nil {
		score *= 1 + cfg.PriorityWeights[int(priority)]
	}
	return score
}

func recordTemporaryCircuitOpened(event channelFailureEvent, reason string, openedAt time.Time) {
	modelName := successRateModelKey(event.ModelName)
	if normalizeCircuitScope(event.CircuitScope) == "channel" {
		modelName = "*"
	}
	if err := model.RecordChannelCircuitEvent(event.ChannelID, successRateGroupKey(event.Group), modelName, event.Reason, reason, openedAt); err != nil {
		common.SysError(fmt.Sprintf("record channel circuit event failed: channel_id=%d group=%s model=%s err=%v", event.ChannelID, successRateGroupKey(event.Group), modelName, err))
	}
}

func recordTemporaryCircuitRecovered(key string, recoveredAt time.Time) {
	parts := strings.Split(key, ":")
	if len(parts) < 3 {
		return
	}
	channelID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || channelID <= 0 {
		return
	}
	groupName := strings.Join(parts[:len(parts)-2], ":")
	modelName := parts[len(parts)-2]
	if err := model.MarkChannelCircuitRecovered(channelID, groupName, modelName, recoveredAt); err != nil {
		common.SysError(fmt.Sprintf("mark channel circuit recovered failed: channel_id=%d group=%s model=%s err=%v", channelID, groupName, modelName, err))
	}
}

func (s *channelSuccessRateSelector) decayLockedForKey(key string, state channelSuccessRateState, now time.Time, cfg channelSuccessRateConfig) (channelSuccessRateState, bool) {
	hadTemporaryCircuit := !state.temporaryOpenUntil.IsZero() || !state.halfOpenUntil.IsZero()
	state = s.decayLocked(state, now, cfg)
	stillInCircuit := !state.temporaryOpenUntil.IsZero() || !state.halfOpenUntil.IsZero()
	return state, hadTemporaryCircuit && !stillInCircuit
}

func (s *channelSuccessRateSelector) isTemporarilyOpen(group string, modelName string, channelID int, now time.Time) bool {
	if s == nil || channelID <= 0 {
		return false
	}
	key := s.circuitKeyForScope(successRateGroupKey(group), modelName, channelID, buildChannelSuccessRateConfig().CircuitScope)
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.circuitState[key]
	state, recovered := s.decayLockedForKey(key, state, now, buildChannelSuccessRateConfig())
	s.circuitState[key] = state
	if recovered {
		recordTemporaryCircuitRecovered(key, now)
	}
	return isTemporaryCircuitOpenAt(state, now)
}

func (s *channelSuccessRateSelector) getTemporaryOpenReason(group string, modelName string, channelID int, now time.Time) string {
	if s == nil || channelID <= 0 {
		return ""
	}
	key := s.circuitKeyForScope(successRateGroupKey(group), modelName, channelID, buildChannelSuccessRateConfig().CircuitScope)
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.circuitState[key]
	state, recovered := s.decayLockedForKey(key, state, now, buildChannelSuccessRateConfig())
	s.circuitState[key] = state
	if recovered {
		recordTemporaryCircuitRecovered(key, now)
	}
	return temporaryCircuitReasonAt(state, now)
}

func (s *channelSuccessRateSelector) buildSelectionDecision(group string, modelName string, channel *model.Channel, cfg channelSuccessRateConfig, now time.Time) selectionDecision {
	decision := selectionDecision{
		channelID: channel.Id,
		name:      channel.Name,
		priority:  channel.GetPriority(),
	}
	if s == nil || channel == nil {
		return decision
	}
	state, circuitKey, recovered, halfOpenBlocked := s.getSelectionState(group, modelName, channel.Id, now, cfg)
	if recovered {
		recordTemporaryCircuitRecovered(circuitKey, now)
	}
	if isTemporaryCircuitOpenAt(state, now) {
		decision.reason = temporaryCircuitReasonAt(state, now)
		return decision
	}
	if halfOpenBlocked {
		decision.reason = halfOpenReasonAt(state, now)
		return decision
	}
	s.mu.Lock()
	decision.score = scoreForStateLocked(s.state[s.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channel.Id)], channel.GetPriority(), cfg)
	s.mu.Unlock()
	decision.reason = fmt.Sprintf("当前权重分数%.4f", decision.score)
	return decision
}

func (s *channelSuccessRateSelector) observe(group string, modelName string, channelID int, success bool, cfg channelSuccessRateConfig) {
	s.observeDetailed(group, modelName, channelID, success, nil, cfg)
}

func (s *channelSuccessRateSelector) observeDetailed(group string, modelName string, channelID int, success bool, err *types.NewAPIError, cfg channelSuccessRateConfig) {
	if s == nil || channelID <= 0 {
		return
	}
	if successRateModelKey(modelName) == "" {
		return
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	key := s.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channelID)
	circuitKey := s.circuitKeyForScope(successRateGroupKey(group), modelName, channelID, cfg.CircuitScope)

	var failureEvent *channelFailureEvent
	s.mu.Lock()
	circuitState, recovered := s.decayLockedForKey(circuitKey, s.circuitState[circuitKey], now, cfg)
	state := s.state[key]
	state.observed += 1
	if success {
		state.success += 1
		state.consecutiveFails = 0
		if isHalfOpenAt(circuitState, now) {
			circuitState.halfOpenProbeInFlight = false
			circuitState.halfOpenSuccesses += 1
			if circuitState.halfOpenSuccessThreshold <= 0 {
				circuitState.halfOpenSuccessThreshold = cfg.HalfOpenSuccessThreshold
			}
			if circuitState.halfOpenSuccesses >= circuitState.halfOpenSuccessThreshold {
				circuitState.halfOpenUntil = time.Time{}
				circuitState.halfOpenReason = ""
				circuitState.halfOpenSuccesses = 0
				circuitState.halfOpenSuccessThreshold = 0
				circuitState.halfOpenProbeInFlight = false
			}
		}
	} else {
		if cfg.QuickDowngrade {
			state.failure += 2
		} else {
			state.failure += 1
		}
		state.consecutiveFails += 1
		if isHalfOpenAt(circuitState, now) {
			circuitState.halfOpenProbeInFlight = false
			circuitState.halfOpenUntil = time.Time{}
			circuitState.halfOpenReason = ""
			circuitState.halfOpenSuccesses = 0
			circuitState.halfOpenSuccessThreshold = 0
		}
		failureEvent = s.buildFailureEventLocked(group, modelName, channelID, state, err, cfg)
	}
	state.updated = now
	s.state[key] = state
	s.circuitState[circuitKey] = circuitState
	if recovered {
		recordTemporaryCircuitRecovered(circuitKey, now)
	}
	s.pruneLocked()
	s.mu.Unlock()

	if failureEvent != nil {
		s.notifyFailure(*failureEvent)
	}
}

func (s *channelSuccessRateSelector) getScore(group string, modelName string, channelID int, cfg channelSuccessRateConfig) float64 {
	if s == nil || channelID <= 0 {
		return 0
	}
	priority := int64(0)
	channel, err := model.CacheGetChannel(channelID)
	if err == nil && channel != nil {
		priority = channel.GetPriority()
	}
	key := s.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channelID)
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.state[key]
	return scoreForStateLocked(state, priority, cfg)
}

func (s *channelSuccessRateSelector) getSampleSize(group string, modelName string, channelID int) int {
	if s == nil || channelID <= 0 {
		return 0
	}
	key := s.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channelID)
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.state[key]
	return state.observed
}

func (s *channelSuccessRateSelector) circuitKeyForScope(group string, modelName string, channelID int, circuitScope string) string {
	modelKey := successRateModelKeyForScope(modelName, circuitScope)
	return group + ":" + modelKey + ":" + fmt.Sprintf("%d", channelID)
}

func (s *channelSuccessRateSelector) scoreKey(group string, modelName string, channelID int) string {
	return group + ":" + modelName + ":" + fmt.Sprintf("%d", channelID)
}

func (s *channelSuccessRateSelector) decayLocked(state channelSuccessRateState, now time.Time, cfg channelSuccessRateConfig) channelSuccessRateState {
	halfLife := time.Duration(cfg.HalfLifeSeconds) * time.Second
	if halfLife <= 0 || state.updated.IsZero() || now.IsZero() || !now.After(state.updated) {
		return transitionCircuitStateOnTime(state, now, cfg)
	}
	delta := now.Sub(state.updated)
	if delta <= 0 {
		return transitionCircuitStateOnTime(state, now, cfg)
	}
	decay := math.Pow(0.5, float64(delta)/float64(halfLife))
	if decay < 0 {
		decay = 0
	}
	if decay > 1 {
		decay = 1
	}
	state.success *= decay
	state.failure *= decay
	state.updated = now
	return transitionCircuitStateOnTime(state, now, cfg)
}

func transitionCircuitStateOnTime(state channelSuccessRateState, now time.Time, cfg channelSuccessRateConfig) channelSuccessRateState {
	if !state.temporaryOpenUntil.IsZero() && !now.Before(state.temporaryOpenUntil) {
		state.temporaryOpenUntil = time.Time{}
		state.temporaryOpenReason = ""
		state.consecutiveFails = 0
		state.halfOpenUntil = now.Add(cfg.RecoveryCheckInterval)
		state.halfOpenReason = "进入半开探测"
		state.halfOpenSuccesses = 0
		state.halfOpenSuccessThreshold = cfg.HalfOpenSuccessThreshold
		state.halfOpenProbeInFlight = false
	}
	if !state.halfOpenUntil.IsZero() && !now.Before(state.halfOpenUntil) {
		state.temporaryOpenUntil = now.Add(cfg.RecoveryCheckInterval)
		state.temporaryOpenReason = "半开探测超时，重新进入临时熔断"
		state.halfOpenUntil = time.Time{}
		state.halfOpenReason = ""
		state.halfOpenSuccesses = 0
		state.halfOpenSuccessThreshold = cfg.HalfOpenSuccessThreshold
		state.halfOpenProbeInFlight = false
	}
	return state
}

func (s *channelSuccessRateSelector) scoreAtLocked(state channelSuccessRateState, priority int64, now time.Time, cfg channelSuccessRateConfig) float64 {
	state = s.decayLocked(state, now, cfg)
	return scoreForStateLocked(state, priority, cfg)
}

func (s *channelSuccessRateSelector) RegisterFailureCallback(cb channelFailureCallback) {
	if s == nil || cb == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failureCallbacks = append(s.failureCallbacks, cb)
}

func (s *channelSuccessRateSelector) buildFailureEventLocked(group string, modelName string, channelID int, state channelSuccessRateState, err *types.NewAPIError, cfg channelSuccessRateConfig) *channelFailureEvent {
	priority := int64(0)
	channel, channelErr := model.CacheGetChannel(channelID)
	if channelErr == nil && channel != nil {
		priority = channel.GetPriority()
	}
	failureEvent := channelFailureEvent{
		Group:            group,
		ModelName:        modelName,
		ChannelID:        channelID,
		Score:            scoreForStateLocked(state, priority, cfg),
		Observed:         state.observed,
		ConsecutiveFails: state.consecutiveFails,
		CircuitScope:     cfg.CircuitScope,
	}
	if err != nil {
		failureEvent.ErrorCode = string(err.GetErrorCode())
		failureEvent.ErrorType = string(err.GetErrorType())
		failureEvent.StatusCode = err.StatusCode
		failureEvent.ErrorMsg = err.ErrorWithStatusCode()
	}
	if cfg.ImmediateDisableEnabled && err != nil {
		if _, ok := cfg.ImmediateDisableStatus[err.StatusCode]; ok {
			failureEvent.Reason = "immediate"
			return &failureEvent
		}
		if _, ok := cfg.ImmediateDisableErrors[string(err.GetErrorCode())]; ok {
			failureEvent.Reason = "immediate"
			return &failureEvent
		}
		if _, ok := cfg.ImmediateDisableTypes[string(err.GetErrorType())]; ok {
			failureEvent.Reason = "immediate"
			return &failureEvent
		}
	}
	if cfg.ConsecutiveFailThreshold > 0 && state.consecutiveFails >= cfg.ConsecutiveFailThreshold {
		failureEvent.Reason = "consecutive"
		return &failureEvent
	}
	return nil
}

func (s *channelSuccessRateSelector) notifyFailure(event channelFailureEvent) {
	if s == nil {
		return
	}
	s.mu.Lock()
	callbacks := make([]channelFailureCallback, len(s.failureCallbacks))
	copy(callbacks, s.failureCallbacks)
	s.mu.Unlock()
	for _, cb := range callbacks {
		cb(event)
	}
}

func (s *channelSuccessRateSelector) ensureRRCursorLocked(key string) int {
	if s.rrCursors == nil {
		s.rrCursors = make(map[string]int)
	}
	if _, ok := s.rrCursors[key]; !ok && len(s.rrCursors) >= s.maxKeyLimit() {
		s.rrCursors = make(map[string]int)
	}
	idx := s.rrCursors[key]
	if idx >= 2_147_483_640 {
		idx = 0
	}
	return idx
}

func (s *channelSuccessRateSelector) pruneLocked() {
	if len(s.state) <= s.maxKeyLimit() && len(s.circuitState) <= s.maxKeyLimit() {
		return
	}
	if len(s.state) > s.maxKeyLimit() {
		s.state = make(map[string]channelSuccessRateState)
	}
	if len(s.circuitState) > s.maxKeyLimit() {
		s.circuitState = make(map[string]channelSuccessRateState)
	}
}

func (s *channelSuccessRateSelector) maxKeyLimit() int {
	if s.maxKeys <= 0 {
		return 4096
	}
	return s.maxKeys
}

type channelSuccessRateConfig struct {
	HalfLifeSeconds          int
	ExploreRate              float64
	QuickDowngrade           bool
	ConsecutiveFailThreshold int
	PriorityWeights          map[int]float64
	ImmediateDisableEnabled  bool
	ImmediateDisableStatus   map[int]struct{}
	ImmediateDisableErrors   map[string]struct{}
	ImmediateDisableTypes    map[string]struct{}
	DisableThreshold         float64
	EnableThreshold          float64
	MinSampleSize            int
	RecoveryCheckInterval    time.Duration
	CircuitScope             string
	HalfOpenSuccessThreshold int
}

func buildChannelSuccessRateConfig() channelSuccessRateConfig {
	setting := operation_setting.GetChannelSuccessRateSetting()
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0.02,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
		PriorityWeights: map[int]float64{
			10: 0.2,
			5:  0,
			0:  -0.1,
		},
		ImmediateDisableEnabled: true,
		ImmediateDisableStatus: map[int]struct{}{
			400: {},
			401: {},
			403: {},
			404: {},
			500: {},
			501: {},
			502: {},
			522: {},
		},
		ImmediateDisableErrors:   make(map[string]struct{}),
		ImmediateDisableTypes:    make(map[string]struct{}),
		DisableThreshold:         0.2,
		EnableThreshold:          0.7,
		MinSampleSize:            10,
		RecoveryCheckInterval:    10 * time.Minute,
		CircuitScope:             "model",
		HalfOpenSuccessThreshold: 2,
	}
	if setting == nil {
		return cfg
	}
	if setting.HalfLifeSeconds > 0 {
		cfg.HalfLifeSeconds = setting.HalfLifeSeconds
	}
	cfg.ExploreRate = setting.ExploreRate
	if cfg.ExploreRate < 0 {
		cfg.ExploreRate = 0
	}
	if cfg.ExploreRate > 1 {
		cfg.ExploreRate = 1
	}
	cfg.QuickDowngrade = setting.QuickDowngrade
	if setting.ConsecutiveFailThreshold > 0 {
		cfg.ConsecutiveFailThreshold = setting.ConsecutiveFailThreshold
	}
	if len(setting.PriorityWeights) > 0 {
		cfg.PriorityWeights = make(map[int]float64, len(setting.PriorityWeights))
		for k, v := range setting.PriorityWeights {
			cfg.PriorityWeights[k] = v
		}
	}
	cfg.ImmediateDisableEnabled = setting.ImmediateDisable.Enabled
	cfg.ImmediateDisableStatus = make(map[int]struct{}, len(setting.ImmediateDisable.StatusCodes))
	for _, code := range setting.ImmediateDisable.StatusCodes {
		cfg.ImmediateDisableStatus[code] = struct{}{}
	}
	cfg.ImmediateDisableErrors = make(map[string]struct{}, len(setting.ImmediateDisable.ErrorCodes))
	for _, code := range setting.ImmediateDisable.ErrorCodes {
		normalized := strings.TrimSpace(code)
		if normalized == "" {
			continue
		}
		cfg.ImmediateDisableErrors[normalized] = struct{}{}
	}
	cfg.ImmediateDisableTypes = make(map[string]struct{}, len(setting.ImmediateDisable.ErrorTypes))
	for _, errorType := range setting.ImmediateDisable.ErrorTypes {
		normalized := strings.TrimSpace(errorType)
		if normalized == "" {
			continue
		}
		cfg.ImmediateDisableTypes[normalized] = struct{}{}
	}
	if setting.HealthManager.DisableThreshold > 0 {
		cfg.DisableThreshold = setting.HealthManager.DisableThreshold
	}
	if setting.HealthManager.EnableThreshold > 0 {
		cfg.EnableThreshold = setting.HealthManager.EnableThreshold
	}
	if setting.HealthManager.MinSampleSize > 0 {
		cfg.MinSampleSize = setting.HealthManager.MinSampleSize
	}
	if setting.HealthManager.RecoveryCheckInterval > 0 {
		cfg.RecoveryCheckInterval = time.Duration(setting.HealthManager.RecoveryCheckInterval) * time.Second
	}
	cfg.CircuitScope = normalizeCircuitScope(setting.HealthManager.CircuitScope)
	if setting.HealthManager.HalfOpenSuccessThreshold > 0 {
		cfg.HalfOpenSuccessThreshold = setting.HealthManager.HalfOpenSuccessThreshold
	}
	return cfg
}

func successRateGroupKey(group string) string {
	return strings.ToLower(strings.TrimSpace(group))
}

func successRateModelKey(modelName string) string {
	return strings.TrimSpace(modelName)
}

func successRateModelKeyForScope(modelName string, circuitScope string) string {
	switch normalizeCircuitScope(circuitScope) {
	case "channel":
		return "*"
	default:
		return successRateModelKey(modelName)
	}
}

func normalizeCircuitScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "channel":
		return "channel"
	default:
		return "model"
	}
}

var channelSuccessRateHealthManagerStartOnce sync.Once

func StartChannelSuccessRateHealthManager() {
	channelSuccessRateHealthManagerStartOnce.Do(func() {
		defaultChannelSuccessRateSelector.RegisterFailureCallback(handleChannelSuccessRateFailure)
		go runChannelSuccessRateRecoveryLoop()
	})
}

func handleChannelSuccessRateFailure(event channelFailureEvent) {
	cfg := buildChannelSuccessRateConfig()
	switch event.Reason {
	case "immediate":
		defaultChannelSuccessRateSelector.openTemporaryCircuit(event, cfg)
	case "consecutive":
		if event.Observed < cfg.MinSampleSize {
			return
		}
		if event.Score < cfg.DisableThreshold {
			defaultChannelSuccessRateSelector.openTemporaryCircuit(event, cfg)
		}
	}
}

func (s *channelSuccessRateSelector) openTemporaryCircuit(event channelFailureEvent, cfg channelSuccessRateConfig) {
	if s == nil || event.ChannelID <= 0 {
		return
	}
	key := s.circuitKeyForScope(successRateGroupKey(event.Group), event.ModelName, event.ChannelID, event.CircuitScope)
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	reason := "连续失败触发临时熔断"
	if event.Reason == "immediate" {
		reason = "命中即时熔断规则"
	}
	if normalizeCircuitScope(event.CircuitScope) == "channel" {
		reason = reason + "（渠道级）"
	}
	if event.ErrorMsg != "" {
		reason = fmt.Sprintf("%s：%s", reason, event.ErrorMsg)
	}
	s.mu.Lock()
	previousState := s.circuitState[key]
	alreadyOpen := isTemporaryCircuitOpenAt(previousState, now)
	state, recovered := s.decayLockedForKey(key, previousState, now, cfg)
	state.temporaryOpenUntil = now.Add(cfg.RecoveryCheckInterval)
	state.temporaryOpenReason = reason
	state.halfOpenUntil = time.Time{}
	state.halfOpenReason = ""
	state.halfOpenSuccesses = 0
	state.halfOpenSuccessThreshold = cfg.HalfOpenSuccessThreshold
	state.halfOpenProbeInFlight = false
	state.updated = now
	s.circuitState[key] = state
	s.mu.Unlock()
	if recovered {
		recordTemporaryCircuitRecovered(key, now)
	}
	if !alreadyOpen {
		recordTemporaryCircuitOpened(event, reason, now)
	}
}

func runChannelSuccessRateRecoveryLoop() {
	for {
		cfg := buildChannelSuccessRateConfig()
		interval := cfg.RecoveryCheckInterval
		if interval <= 0 {
			interval = 10 * time.Minute
		}
		time.Sleep(interval)
		runChannelSuccessRateRecoveryCheck(buildChannelSuccessRateConfig())
	}
}

func runChannelSuccessRateRecoveryCheck(cfg channelSuccessRateConfig) {
	defaultChannelSuccessRateSelector.clearExpiredTemporaryCircuits(cfg)
}

func (s *channelSuccessRateSelector) clearExpiredTemporaryCircuits(cfg channelSuccessRateConfig) {
	if s == nil {
		return
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, state := range s.circuitState {
		state, recovered := s.decayLockedForKey(key, state, now, cfg)
		if recovered {
			recordTemporaryCircuitRecovered(key, now)
		}
		if state.success == 0 && state.failure == 0 && state.observed == 0 && state.temporaryOpenUntil.IsZero() && state.temporaryOpenReason == "" && state.halfOpenUntil.IsZero() && state.halfOpenReason == "" {
			delete(s.circuitState, key)
			continue
		}
		s.circuitState[key] = state
	}
}

func (s *channelSuccessRateSelector) GetRuntimeStateForChannel(channelID int, cfg channelSuccessRateConfig) ChannelSuccessRateRuntimeState {
	if s == nil || channelID <= 0 {
		return ChannelSuccessRateRuntimeState{}
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	best := ChannelSuccessRateRuntimeState{}
	bestObserved := -1
	s.mu.Lock()
	defer s.mu.Unlock()
	for scoreKey, scoreState := range s.state {
		parts := strings.Split(scoreKey, ":")
		if len(parts) < 3 || parts[len(parts)-1] != fmt.Sprintf("%d", channelID) {
			continue
		}
		group := strings.Join(parts[:len(parts)-2], ":")
		modelName := parts[len(parts)-2]
		circuitKey := s.circuitKeyForScope(group, modelName, channelID, cfg.CircuitScope)
		circuitState, recovered := s.decayLockedForKey(circuitKey, s.circuitState[circuitKey], now, cfg)
		s.circuitState[circuitKey] = circuitState
		if recovered {
			recordTemporaryCircuitRecovered(circuitKey, now)
		}
		priority := int64(0)
		channel, err := model.CacheGetChannel(channelID)
		if err == nil && channel != nil {
			priority = channel.GetPriority()
		}
		weightedScore := scoreForStateLocked(scoreState, priority, cfg)
		if override, ok := s.scoreOverrides[channelID]; ok {
			weightedScore = override
		}
		candidate := ChannelSuccessRateRuntimeState{
			TemporaryCircuitOpen:   isTemporaryCircuitOpenAt(circuitState, now),
			TemporaryCircuitUntil:  circuitState.temporaryOpenUntil,
			TemporaryCircuitReason: temporaryCircuitReasonAt(circuitState, now),
			CurrentWeightedScore:   weightedScore,
			Observed:               scoreState.observed,
			Group:                  group,
			ModelName:              modelName,
			HalfOpen:               isHalfOpenAt(circuitState, now),
			HalfOpenUntil:          circuitState.halfOpenUntil,
			HalfOpenReason:         halfOpenReasonAt(circuitState, now),
			HalfOpenSuccesses:      circuitState.halfOpenSuccesses,
			CircuitScope:           cfg.CircuitScope,
		}
		if candidate.Observed > bestObserved {
			best = candidate
			bestObserved = candidate.Observed
		}
	}
	return best
}
