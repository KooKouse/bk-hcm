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
	"hcm/pkg/logs"
	"hcm/pkg/tools/converter"
	"hcm/pkg/tools/slice"

	"github.com/shopspring/decimal"
)

func (b *billItemSvc) exportGcpBillItems(kt *kit.Kit, req *bill.ExportBillItemReq,
	rate *decimal.Decimal) (any, error) {

	result, err := fetchGcpBillItems(kt, b, req)
	if err != nil {
		return nil, err
	}

	regionIDMap := make(map[string]struct{})
	rootAccountIDMap := make(map[string]struct{})
	mainAccountIDMap := make(map[string]struct{})
	bkBizIDMap := make(map[int64]struct{})
	for _, item := range result {
		if item.Extension.GcpRawBillItem != nil {
			regionIDMap[*item.Extension.Region] = struct{}{}
		}
		rootAccountIDMap[item.RootAccountID] = struct{}{}
		mainAccountIDMap[item.MainAccountID] = struct{}{}
		bkBizIDMap[item.BkBizID] = struct{}{}
	}
	rootAccountIDs := converter.MapKeyToSlice(rootAccountIDMap)
	mainAccountIDs := converter.MapKeyToSlice(mainAccountIDMap)
	bkBizIDs := converter.MapKeyToSlice(bkBizIDMap)
	regionIDs := converter.MapKeyToSlice(regionIDMap)

	regionMap, err := b.listGcpRegions(kt, regionIDs)
	if err != nil {
		logs.Errorf("list gcp regions failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	rootAccountMap, mainAccountMap, bizNameMap, err := b.prepareRelatedData(kt, rootAccountIDs,
		mainAccountIDs, bkBizIDs)
	if err != nil {
		logs.Errorf("prepare related data failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, append(commonExcelHeader, gcpExcelHeader...))
	data = append(data, convertGcpBillItem(result, bizNameMap, mainAccountMap, rootAccountMap, regionMap, rate)...)

	buf, err := export.GenerateCSV(data)
	if err != nil {
		logs.Errorf("generate csv failed: %v, data: %v, rid: %s", err, data, kt.Rid)
		return nil, err
	}

	url, err := b.uploadFileAndReturnUrl(kt, buf)
	if err != nil {
		logs.Errorf("upload file failed: %v, rid: %s", err, kt.Rid)
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
			converter.PtrToVal[string](extension.Month),
			bizNameMap[item.BkBizID],
			rootAccountID,
			mainAccountID,
			converter.PtrToVal[string](extension.Region),
			regionMap[converter.PtrToVal[string](extension.Region)],
			converter.PtrToVal[string](extension.ProjectID),
			converter.PtrToVal[string](extension.ProjectName),
			converter.PtrToVal[string](extension.ServiceDescription), // 服务分类
			converter.PtrToVal[string](extension.ServiceDescription), // 服务分类名称
			converter.PtrToVal[string](extension.SkuDescription),
			string(item.Currency),
			converter.PtrToVal[string](extension.UsageUnit),
			(converter.PtrToVal[decimal.Decimal](extension.UsageAmount)).String(),
			item.Cost.String(),
			rate.String(),
			item.Cost.Mul(*rate).String(),
		}

		result = append(result, tmp)
	}
	return result
}

func fetchGcpBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) ([]*billapi.GcpBillItem, error) {

	totalCount, err := fetchGcpBillItemCount(kt, b, req)
	if err != nil {
		logs.Errorf("fetch gcp bill item count failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	limit := totalCount
	if req.ExportLimit <= limit {
		limit = req.ExportLimit
	}

	result := make([]*billapi.GcpBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		billListReq := &databill.BillItemListReq{
			ItemCommonOpt: &databill.ItemCommonOpt{
				Vendor: enumor.Gcp,
				Year:   req.BillYear,
				Month:  req.BillMonth,
			},
			ListReq: &core.ListReq{
				Filter: req.Filter,
				Page: &core.BasePage{
					Start: uint32(offset),
					Limit: page,
				},
			},
		}
		tmpResult, err := b.client.DataService().Gcp.Bill.ListBillItem(kt, billListReq)
		if err != nil {
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func fetchGcpBillItemCount(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) (uint64, error) {
	countReq := &databill.BillItemListReq{
		ItemCommonOpt: &databill.ItemCommonOpt{
			Vendor: enumor.Gcp,
			Year:   req.BillYear,
			Month:  req.BillMonth,
		},
		ListReq: &core.ListReq{Filter: req.Filter, Page: core.NewCountPage()},
	}
	details, err := b.client.DataService().Gcp.Bill.ListBillItem(kt, countReq)
	if err != nil {
		return 0, err
	}
	return details.Count, nil
}

func (b *billItemSvc) listGcpRegions(kt *kit.Kit, regionIDs []string) (map[string]string, error) {
	if len(regionIDs) == 0 {
		return nil, nil
	}
	listReq := &core.ListReq{
		Filter: tools.ExpressionAnd(tools.RuleIn("region_id", slice.Unique(regionIDs))),
		Page:   core.NewDefaultBasePage(),
	}
	regions, err := b.client.DataService().Gcp.Region.ListRegion(kt.Ctx, kt.Header(), listReq)
	if err != nil {
		logs.Errorf("list region failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	regionMap := make(map[string]string)
	for _, region := range regions.Details {
		regionMap[region.RegionID] = region.RegionName
	}
	return regionMap, nil
}
