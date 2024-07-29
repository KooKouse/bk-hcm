package billitem

import (
	"hcm/cmd/account-server/logics/bill/export"
	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	protocore "hcm/pkg/api/core/account-set"
	billapi "hcm/pkg/api/core/bill"
	databill "hcm/pkg/api/data-service/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/kit"
	"hcm/pkg/tools/slice"

	"github.com/shopspring/decimal"
)

func exportGcpBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq, rate *decimal.Decimal) (any, error) {

	result, err := fetchGcpBillItems(kt, b, req)
	if err != nil {
		return nil, err
	}

	regionIDs := make([]string, 0, len(result))
	bkBizIDs := make([]int64, 0, len(result))
	rootAccountIDs := make([]string, 0, len(result))
	mainAccountIDs := make([]string, 0, len(result))
	for _, item := range result {
		if item.Extension.GcpRawBillItem != nil {
			regionIDs = append(regionIDs, *item.Extension.Region)
		}
		bkBizIDs = append(bkBizIDs, item.BkBizID)
		rootAccountIDs = append(rootAccountIDs, item.RootAccountID)
		mainAccountIDs = append(mainAccountIDs, item.MainAccountID)
	}
	// get region info
	regionMap := make(map[string]string)
	if len(regionIDs) > 0 {
		regions, err := b.client.DataService().Gcp.Region.ListRegion(kt.Ctx, kt.Header(), &core.ListReq{
			Filter: tools.ExpressionAnd(tools.RuleIn("region_id", slice.Unique(regionIDs))),
			Page:   core.NewDefaultBasePage(),
		})
		if err != nil {
			return nil, err
		}
		for _, region := range regions.Details {
			regionMap[region.RegionID] = region.RegionName
		}
	}

	mainAccountMap, err := listMainAccount(kt, b, mainAccountIDs)
	if err != nil {
		return nil, err
	}
	rootAccountMap, err := listRootAccount(kt, b, rootAccountIDs)
	if err != nil {
		return nil, err
	}

	bizNameMap, err := listBiz(kt, b, bkBizIDs)
	if err != nil {
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, append(commonExcelHeader, gcpExcelHeader...))
	data = append(data, convertGcpBillItem(result, bizNameMap, mainAccountMap, rootAccountMap, regionMap, rate)...)

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

func convertGcpBillItem(items []*billapi.GcpBillItem, bizNameMap map[int64]string,
	mainAccountMap map[string]*protocore.BaseMainAccount, rootAccountMap map[string]*protocore.BaseRootAccount,
	regionMap map[string]string, rate *decimal.Decimal) [][]string {

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
		extension := item.Extension.GcpRawBillItem
		if extension == nil {
			extension = &billapi.GcpRawBillItem{}
		}

		tmp := []string{
			mainAccountSite,
			safeToString(extension.Month),
			bizNameMap[item.BkBizID],
			rootAccountID,
			mainAccountID,
			safeToString(extension.Region),
			regionMap[safeToString(extension.Region)],
			safeToString(extension.ProjectID),
			safeToString(extension.ProjectName),
			safeToString(extension.ServiceDescription), // 服务分类
			"服务分类名称",
			safeToString(extension.SkuDescription),
			string(item.Currency),
			safeToString(extension.UsageUnit),
			safeToString(extension.UsageAmount),
			item.Cost.String(),
			rate.String(),
			item.Cost.Mul(*rate).String(),
		}

		result = append(result, tmp)
	}
	return result
}

func fetchGcpBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) ([]*billapi.GcpBillItem, error) {

	billListReq := &databill.BillItemListReq{
		ItemCommonOpt: &databill.ItemCommonOpt{
			Vendor: enumor.Gcp,
			Year:   req.BillYear,
			Month:  req.BillMonth,
		},
		ListReq: &core.ListReq{Filter: req.Filter, Page: core.NewCountPage()},
	}
	details, err := b.client.DataService().Gcp.Bill.ListBillItem(kt, billListReq)

	if err != nil {
		return nil, err
	}

	limit := details.Count
	if req.ExportLimit <= limit {
		limit = req.ExportLimit
	}

	result := make([]*billapi.GcpBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		billListReq.Page = &core.BasePage{
			Start: uint32(offset),
			Limit: page,
		}
		tmpResult, err := b.client.DataService().Gcp.Bill.ListBillItem(kt, billListReq)
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}
