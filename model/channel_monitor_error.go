// FORK-CUSTOM: Persist bounded, aggregated error details for channel monitoring.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	channelMonitorErrorBucketDuration = 10 * time.Minute
	channelMonitorErrorRetention      = 72 * time.Hour
	channelMonitorErrorMessageLimit   = 2048
)

var channelMonitorRequestIDRegexp = regexp.MustCompile(`(?i)(request[ _-]?id\s*[:=]?\s*)[a-z0-9_-]+`)

type ChannelMonitorError struct {
	Id              int       `json:"id"`
	ChannelID       int       `json:"channel_id" gorm:"uniqueIndex:idx_xnewapi_monitor_error_bucket,priority:1;index:idx_xnewapi_monitor_error_channel_time,priority:1;not null"`
	TimeBucket      time.Time `json:"time_bucket" gorm:"uniqueIndex:idx_xnewapi_monitor_error_bucket,priority:2;index:idx_xnewapi_monitor_error_channel_time,priority:2;not null"`
	Fingerprint     string    `json:"fingerprint" gorm:"type:char(64);uniqueIndex:idx_xnewapi_monitor_error_bucket,priority:3;not null"`
	ErrorCode       string    `json:"error_code" gorm:"type:varchar(128);not null;default:''"`
	ErrorType       string    `json:"error_type" gorm:"type:varchar(128);not null;default:''"`
	StatusCode      int       `json:"status_code" gorm:"not null;default:0"`
	SampleMessage   string    `json:"sample_message" gorm:"type:text;not null"`
	SampleRequestID string    `json:"sample_request_id" gorm:"type:varchar(64);not null;default:''"`
	FirstOccurredAt int64     `json:"first_occurred_at" gorm:"type:bigint;not null"`
	LastOccurredAt  int64     `json:"last_occurred_at" gorm:"type:bigint;not null"`
	Count           int64     `json:"count" gorm:"type:bigint;not null;default:1"`
	CreatedAt       int64     `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt       int64     `json:"updated_at" gorm:"type:bigint;not null"`
}

func (ChannelMonitorError) TableName() string {
	return "xnewapi_channel_monitor_errors"
}

func RecordChannelMonitorError(channelID int, errorCode string, errorType string, statusCode int, message string, requestID string, occurredAt time.Time) error {
	if DB == nil || channelID <= 0 {
		return nil
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	errorCode = strings.TrimSpace(errorCode)
	errorType = strings.TrimSpace(errorType)
	message = strings.TrimSpace(message)
	messageRunes := []rune(message)
	if len(messageRunes) > channelMonitorErrorMessageLimit {
		message = string(messageRunes[:channelMonitorErrorMessageLimit])
	}
	fingerprintSource := errorCode + "\x00" + errorType + "\x00" + strconv.Itoa(statusCode)
	if errorCode == "" && errorType == "" {
		fingerprintSource += "\x00" + channelMonitorRequestIDRegexp.ReplaceAllString(message, "${1}<request-id>")
	}
	fingerprintBytes := sha256.Sum256([]byte(fingerprintSource))
	nowUnix := occurredAt.Unix()
	errorRecord := &ChannelMonitorError{
		ChannelID: channelID, TimeBucket: occurredAt.Truncate(channelMonitorErrorBucketDuration),
		Fingerprint: hex.EncodeToString(fingerprintBytes[:]), ErrorCode: errorCode,
		ErrorType: errorType, StatusCode: statusCode, SampleMessage: message,
		SampleRequestID: strings.TrimSpace(requestID), FirstOccurredAt: nowUnix,
		LastOccurredAt: nowUnix, Count: 1, CreatedAt: nowUnix, UpdatedAt: nowUnix,
	}
	countColumn := "count"
	firstOccurredAtColumn := "first_occurred_at"
	lastOccurredAtColumn := "last_occurred_at"
	sampleMessageColumn := "sample_message"
	sampleRequestIDColumn := "sample_request_id"
	updatedAtColumn := "updated_at"
	if DB.Dialector.Name() == "postgres" {
		const targetTable = `"xnewapi_channel_monitor_errors".`
		countColumn = targetTable + `"count"`
		firstOccurredAtColumn = targetTable + "first_occurred_at"
		lastOccurredAtColumn = targetTable + "last_occurred_at"
		sampleMessageColumn = targetTable + "sample_message"
		sampleRequestIDColumn = targetTable + "sample_request_id"
		updatedAtColumn = targetTable + "updated_at"
	}
	latestSampleMessage := gorm.Expr("CASE WHEN "+lastOccurredAtColumn+" <= ? THEN ? ELSE "+sampleMessageColumn+" END", nowUnix, message)
	latestSampleRequestID := gorm.Expr("CASE WHEN "+lastOccurredAtColumn+" <= ? THEN ? ELSE "+sampleRequestIDColumn+" END", nowUnix, strings.TrimSpace(requestID))
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "time_bucket"}, {Name: "fingerprint"}},
		DoUpdates: clause.Assignments(map[string]any{
			"count":             gorm.Expr(countColumn+" + ?", 1),
			"first_occurred_at": gorm.Expr("CASE WHEN "+firstOccurredAtColumn+" > ? THEN ? ELSE "+firstOccurredAtColumn+" END", nowUnix, nowUnix),
			"last_occurred_at":  gorm.Expr("CASE WHEN "+lastOccurredAtColumn+" < ? THEN ? ELSE "+lastOccurredAtColumn+" END", nowUnix, nowUnix),
			"sample_message":    latestSampleMessage,
			"sample_request_id": latestSampleRequestID,
			"updated_at":        gorm.Expr("CASE WHEN "+updatedAtColumn+" < ? THEN ? ELSE "+updatedAtColumn+" END", nowUnix, nowUnix),
		}),
	}).Create(errorRecord).Error
}

func CleanupExpiredChannelMonitorErrors(now time.Time) error {
	if DB == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	return DB.Where("time_bucket < ?", now.Add(-channelMonitorErrorRetention)).Delete(&ChannelMonitorError{}).Error
}
