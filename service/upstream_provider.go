// FORK-CUSTOM: Own model-provider validation and credential handling outside the relay path.
package service

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

func ToUpstreamProviderResponse(provider *model.UpstreamProvider) *dto.UpstreamProviderResponse {
	if provider == nil {
		return nil
	}
	response := &dto.UpstreamProviderResponse{
		ID: provider.Id, Name: provider.Name, Endpoint: provider.Endpoint, Status: provider.Status,
		AuthenticationMethod: provider.AuthenticationMethod, AdapterType: provider.AdapterType,
		SyncIntervalMinutes: provider.SyncIntervalMinutes,
		RechargeRatio:       provider.RechargeRatio, QuotaConversionBase: provider.QuotaConversionBase,
		Currency: provider.Currency, SyncStatus: provider.SyncStatus, LastSyncedAt: provider.LastSyncedAt,
		LastSyncError: provider.LastSyncError, CreatedAt: provider.CreatedAt, UpdatedAt: provider.UpdatedAt,
		Accounts: make([]dto.UpstreamProviderAccountResponse, 0, len(provider.Accounts)),
	}
	for _, account := range provider.Accounts {
		response.Accounts = append(response.Accounts, *ToUpstreamProviderAccountResponse(&account))
	}
	return response
}

func ToUpstreamProviderAccountResponse(account *model.UpstreamProviderAccount) *dto.UpstreamProviderAccountResponse {
	if account == nil {
		return nil
	}
	response := &dto.UpstreamProviderAccountResponse{
		ID: account.Id, ProviderID: account.ProviderId, Name: account.Name, ExternalID: account.ExternalId,
		LoginUsername: account.LoginUsername,
		Status:        account.Status, AutoSync: account.AutoSync, HasSyncAPIKey: account.HasSyncAPIKey,
		Balance: account.Balance, TotalRecharge: account.TotalRecharge, BalanceUpdatedAt: account.BalanceUpdatedAt,
		SyncStatus: account.SyncStatus, LastSyncedAt: account.LastSyncedAt, LastSyncAttemptAt: account.LastSyncAttemptAt,
		NextSyncAt: account.NextSyncAt, LastSyncError: account.LastSyncError,
		Groups: make([]dto.UpstreamProviderGroupResponse, 0, len(account.Groups)),
	}
	for _, group := range account.Groups {
		groupResponse := dto.UpstreamProviderGroupResponse{
			ID: group.Id, AccountID: group.AccountId, Name: group.Name,
			ChannelIDs: make([]int, 0, len(group.ChannelMappings)),
			Keys:       make([]dto.UpstreamProviderKeyResponse, 0, len(group.Keys)),
		}
		for _, mapping := range group.ChannelMappings {
			groupResponse.ChannelIDs = append(groupResponse.ChannelIDs, mapping.ChannelId)
		}
		for _, key := range group.Keys {
			groupResponse.Keys = append(groupResponse.Keys, *ToUpstreamProviderKeyResponse(&key))
		}
		response.Groups = append(response.Groups, groupResponse)
	}
	return response
}

func ToUpstreamProviderKeyResponse(key *model.UpstreamProviderKey) *dto.UpstreamProviderKeyResponse {
	if key == nil {
		return nil
	}
	return &dto.UpstreamProviderKeyResponse{
		ID: key.Id, AccountID: key.AccountId, ProviderGroupID: key.ProviderGroupId, Name: key.Name,
		ExternalID: key.ExternalId, KeyMasked: key.KeyMasked, Status: key.Status, LastUsageAt: key.LastUsageAt,
	}
}

func SaveUpstreamProviderWorkspace(request *dto.UpstreamProviderWorkspaceUpsertRequest) (*model.UpstreamProvider, error) {
	if request == nil {
		return nil, errors.New("provider workspace request is required")
	}
	provider, err := buildUpstreamProvider(nil, &request.UpstreamProviderUpsertRequest)
	if err != nil {
		return nil, err
	}
	if request.ID != nil {
		if *request.ID <= 0 {
			return nil, errors.New("invalid provider id")
		}
		provider.Id = *request.ID
	}
	workspace := &model.UpstreamProviderWorkspace{Provider: *provider}
	workspace.Accounts = make([]model.UpstreamProviderWorkspaceAccount, 0, len(request.Accounts))
	accountIDs := make(map[int]struct{}, len(request.Accounts))
	groupIDs := make(map[int]struct{})
	for accountIndex := range request.Accounts {
		accountRequest := &request.Accounts[accountIndex]
		syncCredential := accountRequest.SyncAPIKey
		if provider.AdapterType == "sub2api" {
			syncCredential = accountRequest.LoginPassword
		}
		account, err := buildUpstreamProviderAccount(nil, &dto.UpstreamProviderAccountRequest{
			Name: accountRequest.Name, ExternalId: accountRequest.ExternalId, LoginUsername: accountRequest.LoginUsername, Status: accountRequest.Status,
			AutoSync: accountRequest.AutoSync, SyncAPIKey: syncCredential,
		})
		if err != nil {
			return nil, err
		}
		if accountRequest.ID != nil {
			if *accountRequest.ID <= 0 {
				return nil, errors.New("invalid provider account id")
			}
			if _, exists := accountIDs[*accountRequest.ID]; exists {
				return nil, errors.New("duplicate provider account id")
			}
			accountIDs[*accountRequest.ID] = struct{}{}
			account.Id = *accountRequest.ID
		}
		workspaceAccount := model.UpstreamProviderWorkspaceAccount{
			Account: *account, LoginUsername: accountRequest.LoginUsername, SyncAPIKey: syncCredential,
			Groups: make([]model.UpstreamProviderWorkspaceGroup, 0, len(accountRequest.Groups)),
		}
		for groupIndex := range accountRequest.Groups {
			groupRequest := &accountRequest.Groups[groupIndex]
			groupName := strings.TrimSpace(groupRequest.Name)
			if groupName == "" {
				return nil, errors.New("provider group name is required")
			}
			workspaceGroup := model.UpstreamProviderWorkspaceGroup{
				Group:      model.UpstreamProviderGroup{Name: groupName},
				ChannelIds: groupRequest.ChannelIds,
			}
			if groupRequest.ID != nil {
				if *groupRequest.ID <= 0 {
					return nil, errors.New("invalid provider group id")
				}
				if _, exists := groupIDs[*groupRequest.ID]; exists {
					return nil, errors.New("duplicate provider group id")
				}
				groupIDs[*groupRequest.ID] = struct{}{}
				workspaceGroup.Group.Id = *groupRequest.ID
			}
			workspaceAccount.Groups = append(workspaceAccount.Groups, workspaceGroup)
		}
		workspace.Accounts = append(workspace.Accounts, workspaceAccount)
	}
	saved, err := model.SaveUpstreamProviderWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	if err := refreshProviderProfitDailyDate(time.Now().In(time.Local)); err != nil {
		return nil, err
	}
	return saved, nil
}

func buildUpstreamProvider(provider *model.UpstreamProvider, request *dto.UpstreamProviderUpsertRequest) (*model.UpstreamProvider, error) {
	if request == nil {
		return nil, errors.New("provider request is required")
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, errors.New("provider name is required")
	}
	if request.Status != model.UpstreamProviderStatusEnabled && request.Status != model.UpstreamProviderStatusDisabled {
		return nil, errors.New("provider status must be enabled or disabled")
	}
	adapterType := strings.ToLower(strings.TrimSpace(request.AdapterType))
	if adapterType == "" {
		adapterType = "newapi"
	}
	if !isSupportedProviderAdapter(adapterType) {
		return nil, errors.New("unsupported provider adapter type")
	}
	expectedAuthenticationMethod := "api_key"
	if adapterType == "sub2api" {
		expectedAuthenticationMethod = "password"
	}
	if request.AuthenticationMethod != expectedAuthenticationMethod {
		return nil, errors.New("authentication method does not match provider adapter type")
	}
	if request.SyncIntervalMinutes < 1 || request.SyncIntervalMinutes > 1440 {
		return nil, errors.New("sync interval must be between 1 and 1440 minutes")
	}
	if !request.RechargeRatio.IsPositive() {
		return nil, errors.New("recharge ratio must be positive")
	}
	if adapterType == "newapi" && !request.QuotaConversionBase.IsPositive() {
		return nil, errors.New("quota conversion base must be positive for NewAPI providers")
	}
	currency := strings.ToUpper(strings.TrimSpace(request.Currency))
	if len(currency) != 3 {
		return nil, errors.New("currency must be a three-letter ISO code")
	}
	if provider == nil {
		provider = &model.UpstreamProvider{SyncStatus: "never"}
	}
	provider.Name = name
	provider.Endpoint = strings.TrimSpace(request.Endpoint)
	provider.Status = request.Status
	provider.AuthenticationMethod = request.AuthenticationMethod
	provider.AdapterType = adapterType
	provider.SyncIntervalMinutes = request.SyncIntervalMinutes
	provider.RechargeRatio = request.RechargeRatio
	if adapterType == "sub2api" {
		provider.QuotaConversionBase = decimal.NewFromInt(1)
	} else {
		provider.QuotaConversionBase = request.QuotaConversionBase
	}
	provider.Currency = currency
	return provider, nil
}

func buildUpstreamProviderAccount(account *model.UpstreamProviderAccount, request *dto.UpstreamProviderAccountRequest) (*model.UpstreamProviderAccount, error) {
	if request == nil {
		return nil, errors.New("account request is required")
	}
	if account == nil {
		account = &model.UpstreamProviderAccount{
			Balance:       decimal.Zero,
			TotalRecharge: decimal.Zero,
			SyncStatus:    "never",
		}
	}
	if err := applyUpstreamProviderAccountRequest(account, request.Name, request.ExternalId, request.LoginUsername, request.Status, request.AutoSync, request.SyncAPIKey); err != nil {
		return nil, err
	}
	return account, nil
}

func applyUpstreamProviderAccountRequest(account *model.UpstreamProviderAccount, name string, externalId string, loginUsername *string, status string, autoSync *bool, syncAPIKey *string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("account name is required")
	}
	if status != model.UpstreamProviderStatusEnabled && status != model.UpstreamProviderStatusDisabled {
		return errors.New("account status must be enabled or disabled")
	}
	account.Name = name
	account.ExternalId = strings.TrimSpace(externalId)
	if loginUsername != nil {
		account.LoginUsername = strings.TrimSpace(*loginUsername)
	}
	account.Status = status
	if autoSync != nil {
		account.AutoSync = *autoSync
	}
	if syncAPIKey != nil {
		account.SyncAPIKey = strings.TrimSpace(*syncAPIKey)
	}
	return nil
}
