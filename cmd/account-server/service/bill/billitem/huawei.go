package billitem

import (
	"fmt"

	"hcm/cmd/account-server/logics/bill/export"
	"hcm/pkg/api/core"
	protocore "hcm/pkg/api/core/account-set"
	billapi "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/runtime/filter"

	"github.com/shopspring/decimal"
)

func exportHuaweiBillItems(kt *kit.Kit, b *billItemSvc, filter *filter.Expression,
	requireCount uint64, rate *decimal.Decimal) (any, error) {

	result, err := fetchHuaweiBillItems(kt, b, filter, requireCount)
	if err != nil {
		return nil, err
	}

	rootAccountIDs := make([]string, 0, len(result))
	mainAccountIDs := make([]string, 0, len(result))
	bkBizIDs := make([]int64, 0, len(result))
	for _, item := range result {
		bkBizIDs = append(bkBizIDs, item.BkBizID)
		rootAccountIDs = append(rootAccountIDs, item.RootAccountID)
		mainAccountIDs = append(mainAccountIDs, item.MainAccountID)
	}
	bizNameMap, err := listBiz(kt, b, bkBizIDs)
	if err != nil {
		return nil, err
	}
	mainAccountMap, err := listMainAccount(kt, b, mainAccountIDs)
	if err != nil {
		return nil, err
	}
	rootAccountMap, err := listRootAccount(kt, b, rootAccountIDs)
	if err != nil {
		return nil, err
	}

	data := make([][]interface{}, 0, len(result)+1)
	data = append(data, append(commonExcelTitle, huaWeiExcelTitle...))
	data = append(data, convertHuaweiBillItems(result, bizNameMap, mainAccountMap, rootAccountMap, rate)...)

	buf, err := export.GenerateExcel(data)
	if err != nil {
		return nil, err
	}

	url, err := uploadFileAndReturnUrl(kt, b, buf)
	if err != nil {
		return nil, err
	}

	return url, nil
}

func convertHuaweiBillItems(items []*billapi.HuaweiBillItem, bizNameMap map[int64]string,
	mainAccountMap map[string]*protocore.BaseMainAccount, rootAccountMap map[string]*protocore.BaseRootAccount,
	rate *decimal.Decimal) [][]interface{} {

	result := make([][]interface{}, 0, len(items))
	for _, item := range items {
		var mainAccountID, mainAccountSite, rootAccountID string
		if mainAccount, ok := mainAccountMap[item.MainAccountID]; ok {
			mainAccountID = mainAccount.CloudID
			mainAccountSite = enumor.MainAccountSiteTypeNameMap[mainAccount.Site]
		}
		if rootAccount, ok := rootAccountMap[item.RootAccountID]; ok {
			rootAccountID = rootAccount.CloudID
		}
		var tmp = []interface{}{
			mainAccountSite,
			fmt.Sprintf("%d%02d", item.BillYear, item.BillMonth),
			bizNameMap[item.BkBizID],
			rootAccountID,
			mainAccountID,
			*item.Extension.RegionName,
			*item.Extension.ProductName,
			*item.Extension.Region,
			huaWeiMeasureIdMap[*item.Extension.MeasureId], // 金额单位。 1：元
			*item.Extension.UsageType,
			*item.Extension.UsageMeasureId,
			*item.Extension.CloudServiceType,
			*item.Extension.CloudServiceTypeName,
			*item.Extension.ResourceType,
			*item.Extension.ResourceTypeName,
			huaWeiChargeModeMap[*item.Extension.ChargeMode],
			huaWeiBillTypeMap[*item.Extension.BillType],
			*item.Extension.FreeResourceUsage,
			*item.Extension.Usage,
			*item.Extension.RiUsage,
			item.Currency,
			*rate,
			item.Cost,
			item.Cost.Mul(*rate),
		}
		result = append(result, tmp)
	}
	return result
}

func fetchHuaweiBillItems(kt *kit.Kit, b *billItemSvc, filter *filter.Expression,
	requireCount uint64) ([]*billapi.HuaweiBillItem, error) {

	details, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt,
		&core.ListReq{
			Filter: filter,
			Page:   core.NewCountPage(),
		})

	if err != nil {
		return nil, err
	}

	limit := details.Count
	if requireCount <= limit {
		limit = requireCount
	}

	result := make([]*billapi.HuaweiBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt,
			&core.ListReq{
				Filter: filter,
				Page: &core.BasePage{
					Start: uint32(offset),
					Limit: page,
				},
			})
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}

	return result, nil
}
