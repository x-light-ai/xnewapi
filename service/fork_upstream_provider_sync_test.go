// FORK-CUSTOM: Protect provider credential redaction semantics.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type rotatingProviderSyncAdapter struct{}

func (rotatingProviderSyncAdapter) ValidateAccount(context.Context, *model.UpstreamProvider, *model.UpstreamProviderAccount, ProviderSyncCredentials) error {
	return nil
}

func (rotatingProviderSyncAdapter) RefreshCredentials(_ context.Context, _ *model.UpstreamProvider, _ *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) (ProviderSyncCredentials, error) {
	credentials.AccessToken = "access-new"
	credentials.RefreshToken = "refresh-new"
	return credentials, nil
}

func (rotatingProviderSyncAdapter) SyncAccountSnapshot(context.Context, *model.UpstreamProvider, *model.UpstreamProviderAccount, ProviderSyncCredentials) (*ProviderAccountSnapshot, error) {
	return nil, errors.New("snapshot failed after refresh")
}

func TestSanitizeProviderSyncError(t *testing.T) {
	err := errors.New("upstream rejected Authorization Bearer secret-value")
	message := sanitizeProviderSyncError(err, "secret-value")
	assert.Equal(t, "upstream rejected Authorization Bearer [redacted]", message)
}

func TestSyncUpstreamProviderAccountKeepsRotatedTokensWhenSnapshotFails(t *testing.T) {
	originalDB := model.DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, originalLogType)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&model.UpstreamProvider{}, &model.UpstreamProviderAccount{}, &model.UpstreamProviderSyncRun{}))

	provider := &model.UpstreamProvider{
		Name: "provider", Status: model.UpstreamProviderStatusEnabled, AuthenticationMethod: "token", AdapterType: "sub2api",
		SyncIntervalMinutes: 60, SyncStatus: "never",
	}
	require.NoError(t, db.Create(provider).Error)
	account := &model.UpstreamProviderAccount{
		ProviderId: provider.Id, Name: "account", Status: model.UpstreamProviderStatusEnabled,
		SyncStatus: "never", AccessToken: "access-old", RefreshToken: "refresh-old",
	}
	require.NoError(t, db.Create(account).Error)

	run, syncErr := syncUpstreamProviderAccount(context.Background(), rotatingProviderSyncAdapter{}, provider, account)
	require.EqualError(t, syncErr, "snapshot failed after refresh")
	require.NotNil(t, run)
	assert.Equal(t, "error", run.Status)

	var stored model.UpstreamProviderAccount
	require.NoError(t, db.First(&stored, account.Id).Error)
	assert.Equal(t, "access-new", stored.AccessToken)
	assert.Equal(t, "refresh-new", stored.RefreshToken)
}
