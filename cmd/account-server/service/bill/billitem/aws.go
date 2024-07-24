package billitem

import (
	"fmt"

	"hcm/cmd/account-server/logics/bill/export"
	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	protocore "hcm/pkg/api/core/account-set"
	billapi "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/kit"
	"hcm/pkg/runtime/filter"

	"github.com/shopspring/decimal"
)

func exportAwsBillItems(kt *kit.Kit, b *billItemSvc, filter *filter.Expression,
	requireCount uint64, rate *decimal.Decimal) (any, error) {

	result, err := fetchAwsBillItems(kt, b, filter, requireCount)
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
	data = append(data, append(commonExcelHeader, awsExcelHeader...))
	data = append(data, convertAwsBillItems(result, bizNameMap, mainAccountMap, rootAccountMap, rate)...)

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

func convertAwsBillItems(items []*billapi.AwsBillItem, bizNameMap map[int64]string,
	mainAccountMap map[string]*protocore.BaseMainAccount, rootAccountMap map[string]*protocore.BaseRootAccount,
	rate *decimal.Decimal) [][]string {

	result := make([][]string, 0, len(items))
	for _, item := range items {
		var mainAccountID, rootAccountID, mainAccountSite string
		if mainAccount, ok := mainAccountMap[item.MainAccountID]; ok {
			mainAccountID = mainAccount.CloudID
			mainAccountSite = enumor.MainAccountSiteTypeNameMap[mainAccount.Site]
		}
		if rootAccount, ok := rootAccountMap[item.RootAccountID]; ok {
			rootAccountID = rootAccount.CloudID
		}

		tmp := []string{
			mainAccountSite,
			fmt.Sprintf("%d-%02d", item.BillYear, item.BillMonth),
			bizNameMap[item.BkBizID],
			rootAccountID,
			mainAccountID,
			item.Extension.ProductToRegionCode,
			item.Extension.ProductFromLocation,
			item.Extension.BillInvoiceId,
			item.Extension.BillBillingEntity,
			item.Extension.LineItemProductCode,
			item.Extension.ProductProductFamily,
			item.Extension.ProductProductName,
			"API操作",
			"产品规格",
			"实例类型 product_instance_type?",
			item.Extension.LineItemResourceId,
			item.Extension.PricingTerm,
			item.Extension.LineItemLineItemType,
			item.Extension.LineItemLineItemDescription,
			item.Extension.LineItemUsageAmount,
			item.Extension.PricingUnit,
			item.Cost.String(),
			string(item.Currency),
			item.Cost.Mul(*rate).String(),
			rate.String(),
		}
		result = append(result, tmp)
	}
	return result
}

func fetchAwsBillItems(kt *kit.Kit, b *billItemSvc, filter *filter.Expression,
	requireCount uint64) ([]*billapi.AwsBillItem, error) {

	details, err := b.client.DataService().Aws.Bill.ListBillItem(kt,
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

	result := make([]*billapi.AwsBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := b.client.DataService().Aws.Bill.ListBillItem(kt,
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
