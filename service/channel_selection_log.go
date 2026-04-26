package service

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const channelSelectionLogBufferSize = 100

const (
	ChannelSelectionLogEventSelection   = "selection"
	ChannelSelectionLogEventObservation = "observation"

	ChannelSelectionOutcomeSelected         = "selected"
	ChannelSelectionOutcomeNoAvailable      = "no_available"
	ChannelSelectionOutcomeTemporaryCircuit = "temporary_circuit"
	ChannelSelectionOutcomeHalfOpenBlocked  = "half_open_blocked"
	ChannelSelectionOutcomeRequestSuccess   = "request_success"
	ChannelSelectionOutcomeRequestFailed    = "request_failed"
)

type ChannelSelectionLogCandidate struct {
	ChannelID int     `json:"channel_id"`
	Name      string  `json:"name"`
	Priority  int64   `json:"priority"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
}

type ChannelSelectionLogEntry struct {
	Timestamp           time.Time                      `json:"timestamp"`
	EventType           string                         `json:"event_type"`
	Outcome             string                         `json:"outcome"`
	OutcomeDetail       string                         `json:"outcome_detail"`
	ModelName           string                         `json:"model_name"`
	Group               string                         `json:"group"`
	SelectedChannelID   int                            `json:"selected_channel_id"`
	SelectedChannelName string                         `json:"selected_channel_name"`
	SelectedPriority    int64                          `json:"selected_priority"`
	SelectedScore       float64                        `json:"selected_score"`
	CandidateCount      int                            `json:"candidate_count"`
	HasCircuit          bool                           `json:"has_circuit"`
	CircuitCount        int                            `json:"circuit_count"`
	Summary             string                         `json:"summary"`
	Candidates          []ChannelSelectionLogCandidate `json:"candidates"`
}

type ChannelSelectionLogFilter struct {
	ChannelID    int
	ModelName    string
	Group        string
	Outcome      string
	AbnormalOnly bool
	Limit        int
}

type channelSelectionLogBuffer struct {
	mu      sync.RWMutex
	entries []ChannelSelectionLogEntry
}

var defaultChannelSelectionLogBuffer = newChannelSelectionLogBuffer(channelSelectionLogBufferSize)

func newChannelSelectionLogBuffer(capacity int) *channelSelectionLogBuffer {
	if capacity <= 0 {
		capacity = channelSelectionLogBufferSize
	}
	return &channelSelectionLogBuffer{
		entries: make([]ChannelSelectionLogEntry, 0, capacity),
	}
}

func (b *channelSelectionLogBuffer) append(entry ChannelSelectionLogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= cap(b.entries) {
		copy(b.entries, b.entries[1:])
		b.entries[len(b.entries)-1] = entry
		return
	}
	b.entries = append(b.entries, entry)
}

func (b *channelSelectionLogBuffer) list(filter ChannelSelectionLogFilter) []ChannelSelectionLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 || limit > cap(b.entries) {
		limit = cap(b.entries)
	}
	result := make([]ChannelSelectionLogEntry, 0, minInt(limit, len(b.entries)))
	for i := len(b.entries) - 1; i >= 0; i-- {
		entry := b.entries[i]
		if !channelSelectionLogMatchesFilter(entry, filter) {
			continue
		}
		result = append(result, cloneChannelSelectionLogEntry(entry))
		if len(result) >= limit {
			break
		}
	}
	return result
}

func channelSelectionLogMatchesFilter(entry ChannelSelectionLogEntry, filter ChannelSelectionLogFilter) bool {
	if filter.ChannelID > 0 && !channelSelectionLogContainsChannel(entry, filter.ChannelID) {
		return false
	}
	if filter.ModelName != "" && !strings.Contains(strings.ToLower(entry.ModelName), strings.ToLower(filter.ModelName)) {
		return false
	}
	if filter.Group != "" && entry.Group != filter.Group {
		return false
	}
	if filter.Outcome != "" && filter.Outcome != "all" && entry.Outcome != filter.Outcome {
		return false
	}
	if filter.AbnormalOnly && !channelSelectionLogIsAbnormal(entry) {
		return false
	}
	return true
}

func channelSelectionLogIsAbnormal(entry ChannelSelectionLogEntry) bool {
	if entry.HasCircuit {
		return true
	}
	switch entry.Outcome {
	case ChannelSelectionOutcomeNoAvailable, ChannelSelectionOutcomeTemporaryCircuit, ChannelSelectionOutcomeHalfOpenBlocked, ChannelSelectionOutcomeRequestFailed:
		return true
	default:
		return false
	}
}

func channelSelectionLogContainsChannel(entry ChannelSelectionLogEntry, channelID int) bool {
	if entry.SelectedChannelID == channelID {
		return true
	}
	for _, candidate := range entry.Candidates {
		if candidate.ChannelID == channelID {
			return true
		}
	}
	return false
}

func cloneChannelSelectionLogEntry(entry ChannelSelectionLogEntry) ChannelSelectionLogEntry {
	cloned := entry
	if len(entry.Candidates) > 0 {
		cloned.Candidates = append([]ChannelSelectionLogCandidate(nil), entry.Candidates...)
	}
	return cloned
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildSelectionSummary(others []selectionDecision) string {
	parts := make([]string, 0, len(others))
	for i, item := range others {
		if i >= 5 {
			parts = append(parts, fmt.Sprintf("其余%d个渠道已省略", len(others)-i))
			break
		}
		parts = append(parts, formatSelectionDecision(item))
	}
	if len(parts) == 0 {
		return "无"
	}
	return joinSelectionSummary(parts)
}

func joinSelectionSummary(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "；"
		}
		result += part
	}
	return result
}

func appendChannelSelectionLog(now time.Time, modelName string, group string, candidateCount int, selectedID int, selectedName string, selectedPriority int64, selectedScore float64, others []selectionDecision) {
	summary := buildSelectionSummary(others)
	outcome, outcomeDetail := classifyChannelSelectionOutcome(selectedID, summary, others)
	entry := ChannelSelectionLogEntry{
		Timestamp:           now,
		EventType:           ChannelSelectionLogEventSelection,
		Outcome:             outcome,
		OutcomeDetail:       outcomeDetail,
		ModelName:           modelName,
		Group:               group,
		SelectedChannelID:   selectedID,
		SelectedChannelName: selectedName,
		SelectedPriority:    selectedPriority,
		SelectedScore:       selectedScore,
		CandidateCount:      candidateCount,
		Summary:             summary,
		Candidates:          make([]ChannelSelectionLogCandidate, 0, len(others)+1),
	}
	if selectedID > 0 {
		entry.Candidates = append(entry.Candidates, ChannelSelectionLogCandidate{
			ChannelID: selectedID,
			Name:      selectedName,
			Priority:  selectedPriority,
			Score:     selectedScore,
			Reason:    "已选中",
		})
	}
	for _, item := range others {
		candidate := ChannelSelectionLogCandidate{
			ChannelID: item.channelID,
			Name:      item.name,
			Priority:  item.priority,
			Score:     item.score,
			Reason:    item.reason,
		}
		if channelSelectionReasonHasCircuit(candidate.Reason) {
			entry.HasCircuit = true
			entry.CircuitCount++
		}
		entry.Candidates = append(entry.Candidates, candidate)
	}
	defaultChannelSelectionLogBuffer.append(entry)
}

func appendChannelSelectionObservationLog(now time.Time, modelName string, group string, channelID int, channelName string, priority int64, score float64, success bool, detail string, circuitReason string) {
	outcome := ChannelSelectionOutcomeRequestSuccess
	if !success {
		outcome = ChannelSelectionOutcomeRequestFailed
	}
	if strings.TrimSpace(detail) == "" {
		if success {
			detail = "请求成功"
		} else {
			detail = "请求失败"
		}
	}
	if circuitReason != "" {
		detail = detail + "；" + circuitReason
	}
	entry := ChannelSelectionLogEntry{
		Timestamp:           now,
		EventType:           ChannelSelectionLogEventObservation,
		Outcome:             outcome,
		OutcomeDetail:       detail,
		ModelName:           modelName,
		Group:               group,
		SelectedChannelID:   channelID,
		SelectedChannelName: channelName,
		SelectedPriority:    priority,
		SelectedScore:       score,
		CandidateCount:      1,
		HasCircuit:          circuitReason != "",
		Summary:             detail,
		Candidates: []ChannelSelectionLogCandidate{{
			ChannelID: channelID,
			Name:      channelName,
			Priority:  priority,
			Score:     score,
			Reason:    detail,
		}},
	}
	if entry.HasCircuit {
		entry.CircuitCount = 1
	}
	defaultChannelSelectionLogBuffer.append(entry)
}

func classifyChannelSelectionOutcome(selectedID int, summary string, others []selectionDecision) (string, string) {
	if selectedID > 0 {
		if channelSelectionDecisionsHaveCircuit(others) {
			return ChannelSelectionOutcomeSelected, "已选中，候选中存在熔断或半开阻断渠道"
		}
		return ChannelSelectionOutcomeSelected, "已选中"
	}
	if channelSelectionSummaryHasHalfOpen(summary) {
		return ChannelSelectionOutcomeHalfOpenBlocked, summary
	}
	if channelSelectionDecisionsHaveCircuit(others) {
		return ChannelSelectionOutcomeTemporaryCircuit, summary
	}
	return ChannelSelectionOutcomeNoAvailable, summary
}

func channelSelectionDecisionsHaveCircuit(others []selectionDecision) bool {
	for _, item := range others {
		if channelSelectionReasonHasCircuit(item.reason) {
			return true
		}
	}
	return false
}

func channelSelectionReasonHasCircuit(reason string) bool {
	return strings.Contains(reason, "熔断") || strings.Contains(reason, "半开")
}

func channelSelectionSummaryHasHalfOpen(summary string) bool {
	return strings.Contains(summary, "半开")
}

func ListChannelSelectionLogs(filter ChannelSelectionLogFilter) []ChannelSelectionLogEntry {
	return defaultChannelSelectionLogBuffer.list(filter)
}
