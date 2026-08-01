// FORK-CUSTOM: Expose model-provider management APIs from the fork route boundary.
package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetUpstreamProviders(c *gin.Context) {
	page := common.GetPageQuery(c)
	providers, total, err := model.GetUpstreamProviderPage(page.GetStartIdx(), page.GetPageSize(), c.Query("keyword"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]*dto.UpstreamProviderResponse, 0, len(providers))
	for index := range providers {
		items = append(items, service.ToUpstreamProviderResponse(&providers[index]))
	}
	page.SetTotal(int(total))
	page.SetItems(items)
	common.ApiSuccess(c, page)
}

func GetUpstreamProviderChannelOptions(c *gin.Context) {
	channels, err := model.GetUpstreamProviderChannelOptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, channels)
}

func SaveUpstreamProviderWorkspace(c *gin.Context) {
	var request dto.UpstreamProviderWorkspaceUpsertRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	provider, err := service.SaveUpstreamProviderWorkspace(&request)
	if err != nil {
		if errors.Is(err, model.ErrUpstreamProviderChannelMapped) {
			c.JSON(409, gin.H{"success": false, "message": err.Error()})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, service.ToUpstreamProviderResponse(provider))
}

func SyncUpstreamProvider(c *gin.Context) {
	id, err := upstreamProviderRouteID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	runs, err := service.SyncUpstreamProvider(c.Request.Context(), id, 0, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, runs)
}

func SyncUpstreamProviderAccount(c *gin.Context) {
	accountId, err := upstreamProviderAccountRouteID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	account, err := model.GetUpstreamProviderAccountById(accountId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	runs, err := service.SyncUpstreamProvider(c.Request.Context(), account.ProviderId, account.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, runs)
}

func AdjustUpstreamProviderAccountRecharge(c *gin.Context) {
	accountID, err := upstreamProviderAccountRouteID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request dto.UpstreamProviderAccountRechargeAdjustRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Delta.IsZero() {
		common.ApiErrorMsg(c, "recharge adjustment must not be zero")
		return
	}
	if request.Delta.Exponent() < -8 {
		common.ApiErrorMsg(c, "recharge adjustment supports at most 8 decimal places")
		return
	}
	account, err := model.AdjustUpstreamProviderAccountRecharge(accountID, request.Delta)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, service.ToUpstreamProviderAccountResponse(account))
}

func DeleteUpstreamProvider(c *gin.Context) {
	id, err := upstreamProviderRouteID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteUpstreamProvider(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func DeleteUpstreamProviderAccount(c *gin.Context) {
	accountID, err := upstreamProviderAccountRouteID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteUpstreamProviderAccount(accountID); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func DeleteUpstreamProviderGroup(c *gin.Context) {
	groupID, err := upstreamProviderGroupRouteID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteUpstreamProviderGroup(groupID); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, true)
}

func GetUpstreamProviderSyncRuns(c *gin.Context) {
	providerId, _ := strconv.Atoi(c.Query("provider_id"))
	accountId, _ := strconv.Atoi(c.Query("account_id"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	runs, err := model.GetUpstreamProviderSyncRuns(providerId, accountId, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, runs)
}

func GetUpstreamProviderProfitRanking(c *gin.Context) {
	items, err := service.GetProviderGroupProfitRanking(c.Query("start_date"), c.Query("end_date"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func RebuildUpstreamProviderProfit(c *gin.Context) {
	var request dto.UpstreamProviderProfitQuery
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.RebuildProviderProfitDaily(request.StartDate, request.EndDate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func upstreamProviderRouteID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid provider id")
	}
	return id, nil
}

func upstreamProviderAccountRouteID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("account_id")))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid provider account id")
	}
	return id, nil
}

func upstreamProviderGroupRouteID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("group_id")))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid provider group id")
	}
	return id, nil
}
