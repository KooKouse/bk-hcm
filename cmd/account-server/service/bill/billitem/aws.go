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
	"hcm/pkg/logs"
	"hcm/pkg/tools/converter"

	"github.com/shopspring/decimal"
)

func (b *billItemSvc) exportAwsBillItems(kt *kit.Kit, req *bill.ExportBillItemReq,
	rate *decimal.Decimal) (any, error) {

	result, err := fetchAwsBillItems(kt, b, req)
	if err != nil {
		logs.Errorf("fetch aws bill items  for export failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	rootAccountIDMap := make(map[string]struct{})
	mainAccountIDMap := make(map[string]struct{})
	bkBizIDMap := make(map[int64]struct{})
	for _, item := range result {
		rootAccountIDMap[item.RootAccountID] = struct{}{}
		mainAccountIDMap[item.MainAccountID] = struct{}{}
		bkBizIDMap[item.BkBizID] = struct{}{}
	}
	rootAccountIDs := converter.MapKeyToSlice(rootAccountIDMap)
	mainAccountIDs := converter.MapKeyToSlice(mainAccountIDMap)
	bkBizIDs := converter.MapKeyToSlice(bkBizIDMap)
	rootAccountMap, mainAccountMap, bizNameMap, err := b.fetchAccountBizInfo(kt, rootAccountIDs,
		mainAccountIDs, bkBizIDs)
	if err != nil {
		logs.Errorf("prepare related data failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, append(commonExcelHeader, awsExcelHeader...))
	table, err := convertAwsBillItems(result, bizNameMap, mainAccountMap, rootAccountMap, rate)
	if err != nil {
		logs.Errorf("convert to raw data error: %s, rid: %s", err, kt.Rid)
		return nil, err
	}
	data = append(data, table...)

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

func convertAwsBillItems(items []*billapi.AwsBillItem, bizNameMap map[int64]string,
	mainAccountMap map[string]*protocore.BaseMainAccount, rootAccountMap map[string]*protocore.BaseRootAccount,
	rate *decimal.Decimal) ([][]string, error) {

	result := make([][]string, 0, len(items))
	for _, item := range items {
		mainAccount, ok := mainAccountMap[item.MainAccountID]
		if !ok {
			return nil, fmt.Errorf("main account(%s) not found", item.MainAccountID)
		}
		rootAccount, ok := rootAccountMap[item.RootAccountID]
		if !ok {
			return nil, fmt.Errorf("root account(%s) not found", item.RootAccountID)
		}

		extension := item.Extension.AwsRawBillItem
		if extension == nil {
			extension = &billapi.AwsRawBillItem{}
		}

		tmp := []string{
			string(mainAccount.Site),
			fmt.Sprintf("%d-%02d", item.BillYear, item.BillMonth),
			bizNameMap[item.BkBizID],
			rootAccount.ID,
			mainAccount.ID,
			extension.ProductToRegionCode,
			extension.ProductFromLocation,
			extension.BillInvoiceId,
			extension.BillBillingEntity,
			extension.LineItemProductCode,
			extension.ProductProductFamily,
			extension.ProductProductName,
			extension.LineItemOperation, // line_item_operation
			extension.ProductUsagetype,
			extension.ProductInsightstype,
			extension.LineItemResourceId,
			extension.PricingTerm,
			extension.LineItemLineItemType,
			extension.LineItemLineItemDescription,
			extension.LineItemUsageAmount,
			extension.PricingUnit,
			item.Cost.String(),
			string(item.Currency),
			item.Cost.Mul(*rate).String(),
			rate.String(),
		}
		result = append(result, tmp)
	}
	return result, nil
}

func fetchAwsBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) ([]*billapi.AwsBillItem, error) {

	totalCount, err := fetchAwsBillItemCount(kt, b, req)
	if err != nil {
		logs.Errorf("fetch aws bill item count failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	exportLimit := min(totalCount, req.ExportLimit)

	commonOpt := &databill.ItemCommonOpt{
		Vendor: enumor.Aws,
		Year:   req.BillYear,
		Month:  req.BillMonth,
	}
	result := make([]*billapi.AwsBillItem, 0, exportLimit)
	for offset := uint64(0); offset < exportLimit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		left := exportLimit - offset
		billListReq := &databill.BillItemListReq{
			ItemCommonOpt: commonOpt,
			ListReq: &core.ListReq{
				Filter: req.Filter,
				Page: &core.BasePage{
					Start: uint32(offset),
					Limit: min(uint(left), core.DefaultMaxPageLimit),
				},
			},
		}
		tmpResult, err := b.client.DataService().Aws.Bill.ListBillItem(kt, billListReq)
		if err != nil {
			logs.Errorf("list aws bill item failed: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func fetchAwsBillItemCount(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) (uint64, error) {
	countReq := &databill.BillItemListReq{
		ItemCommonOpt: &databill.ItemCommonOpt{
			Vendor: enumor.Aws,
			Year:   req.BillYear,
			Month:  req.BillMonth,
		},
		ListReq: &core.ListReq{Filter: req.Filter, Page: core.NewCountPage()},
	}
	details, err := b.client.DataService().Aws.Bill.ListBillItem(kt, countReq)
	if err != nil {
		return 0, err
	}
	return details.Count, nil
}
