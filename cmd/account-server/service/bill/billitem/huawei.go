package billitem

import (
	"fmt"

	"hcm/cmd/account-server/logics/bill/export"
	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	protocore "hcm/pkg/api/core/account-set"
	billapi "hcm/pkg/api/core/bill"
	databill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"

	"github.com/TencentBlueKing/gopkg/conv"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/bssintl/v2/model"
	"github.com/shopspring/decimal"
)

func exportHuaweiBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq, rate *decimal.Decimal) (any, error) {

	result, err := fetchHuaweiBillItems(kt, b, req)
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

	data := make([][]string, 0, len(result)+1)
	data = append(data, append(commonExcelHeader, huaWeiExcelHeader...))
	data = append(data, convertHuaweiBillItems(result, bizNameMap, mainAccountMap, rootAccountMap, rate)...)

	buf, err := export.GenerateCSV(data)
	if err != nil {
		return nil, err
	}

	url, err := uploadFileAndReturnUrl(kt, b, buf)
	if err != nil {
		return nil, err
	}

	return bill.BillExportResult{DownloadURL: url}, nil
}

func convertHuaweiBillItems(items []*billapi.HuaweiBillItem, bizNameMap map[int64]string,
	mainAccountMap map[string]*protocore.BaseMainAccount, rootAccountMap map[string]*protocore.BaseRootAccount,
	rate *decimal.Decimal) [][]string {

	result := make([][]string, 0, len(items))
	for _, item := range items {
		var mainAccountID, mainAccountSite, rootAccountID string
		if mainAccount, ok := mainAccountMap[item.MainAccountID]; ok {
			mainAccountID = mainAccount.CloudID
			mainAccountSite = enumor.MainAccountSiteTypeNameMap[mainAccount.Site]
		}
		if rootAccount, ok := rootAccountMap[item.RootAccountID]; ok {
			rootAccountID = rootAccount.CloudID
		}

		extension := item.Extension.ResFeeRecordV2
		if extension == nil {
			extension = &model.ResFeeRecordV2{}
		}

		var tmp = []string{
			mainAccountSite,
			fmt.Sprintf("%d%02d", item.BillYear, item.BillMonth),
			bizNameMap[item.BkBizID],
			rootAccountID,
			mainAccountID,
			safeToString(extension.RegionName),
			safeToString(extension.ProductName),
			safeToString(extension.Region),
			huaWeiMeasureIdMap[int32(safeToInt(extension.MeasureId))], // 金额单位。 1：元
			safeToString(extension.UsageType),
			conv.ToString(safeToString(extension.UsageMeasureId)),
			safeToString(extension.CloudServiceType),
			safeToString(extension.CloudServiceTypeName),
			safeToString(extension.ResourceType),
			safeToString(extension.ResourceTypeName),
			huaWeiChargeModeMap[safeToString(extension.ChargeMode)],
			huaWeiBillTypeMap[int32(safeToInt(extension.BillType))],
			conv.ToString(safeToString(extension.FreeResourceUsage)),
			conv.ToString(safeToString(extension.Usage)),
			conv.ToString(safeToString(extension.RiUsage)),
			string(item.Currency),
			rate.String(),
			item.Cost.String(),
			item.Cost.Mul(*rate).String(),
		}
		result = append(result, tmp)
	}
	return result
}

func fetchHuaweiBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) (
	[]*billapi.HuaweiBillItem, error) {

	billListReq := &databill.BillItemListReq{
		ItemCommonOpt: &databill.ItemCommonOpt{
			Vendor: enumor.HuaWei,
			Year:   req.BillYear,
			Month:  req.BillMonth,
		},
		ListReq: &core.ListReq{Filter: req.Filter, Page: core.NewCountPage()},
	}
	details, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt, billListReq)
	if err != nil {
		return nil, err
	}

	limit := details.Count
	if req.ExportLimit <= limit {
		limit = req.ExportLimit
	}

	result := make([]*billapi.HuaweiBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		billListReq.Page = &core.BasePage{
			Start: uint32(offset),
			Limit: page,
		}
		tmpResult, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt, billListReq)
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}

	return result, nil
}
