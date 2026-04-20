package model

import "time"

type ChannelCircuitEvent struct {
	Id          int       `json:"id"`
	ChannelID   int       `json:"channel_id" gorm:"index:idx_channel_circuit_events_channel_opened_at,priority:1;not null"`
	GroupName   string    `json:"group_name" gorm:"type:varchar(64);not null;default:''"`
	ModelName   string    `json:"model_name" gorm:"type:varchar(128);not null;default:''"`
	CircuitType string    `json:"circuit_type" gorm:"type:varchar(32);not null;default:'temporary'"`
	TriggerType string    `json:"trigger_type" gorm:"type:varchar(32);not null;default:''"`
	Reason      string    `json:"reason" gorm:"type:text;not null"`
	OpenedAt    int64     `json:"opened_at" gorm:"bigint;index:idx_channel_circuit_events_channel_opened_at,priority:2;index:idx_channel_circuit_events_opened_at"`
	RecoveredAt int64     `json:"recovered_at" gorm:"bigint;default:0"`
	CreatedAt   int64     `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64     `json:"updated_at" gorm:"bigint"`
}

func (ChannelCircuitEvent) TableName() string {
	return "channel_circuit_events"
}

func NewChannelCircuitEvent(channelID int, groupName string, modelName string, triggerType string, reason string, openedAt time.Time) *ChannelCircuitEvent {
	openedAtUnix := openedAt.Unix()
	if openedAtUnix <= 0 {
		openedAtUnix = time.Now().Unix()
	}
	return &ChannelCircuitEvent{
		ChannelID:   channelID,
		GroupName:   groupName,
		ModelName:   modelName,
		CircuitType: "temporary",
		TriggerType: triggerType,
		Reason:      reason,
		OpenedAt:    openedAtUnix,
		CreatedAt:   openedAtUnix,
		UpdatedAt:   openedAtUnix,
	}
}

func RecordChannelCircuitEvent(channelID int, groupName string, modelName string, triggerType string, reason string, openedAt time.Time) error {
	event := NewChannelCircuitEvent(channelID, groupName, modelName, triggerType, reason, openedAt)
	return DB.Create(event).Error
}

func MarkChannelCircuitRecovered(channelID int, groupName string, modelName string, recoveredAt time.Time) error {
	recoveredAtUnix := recoveredAt.Unix()
	if recoveredAtUnix <= 0 {
		recoveredAtUnix = time.Now().Unix()
	}
	return DB.Model(&ChannelCircuitEvent{}).
		Where("channel_id = ? AND group_name = ? AND model_name = ? AND recovered_at = 0", channelID, groupName, modelName).
		Updates(map[string]any{
			"recovered_at": recoveredAtUnix,
			"updated_at":   recoveredAtUnix,
		}).Error
}
