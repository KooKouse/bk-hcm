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

	"github.com/TencentBlueKing/gopkg/conv"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/bssintl/v2/model"
	"github.com/shopspring/decimal"
)

var (
	// 金额单位。 1：元
	huaWeiMeasureIdMap = map[int32]string{
		1: "元",
	}

	// 计费模式。 1：包年/包月3：按需10：预留实例
	huaWeiChargeModeMap = map[string]string{
		"1":  "包年/包月",
		"3":  "按需",
		"10": "预留实例",
	}

	// 账单类型
	huaWeiBillTypeMap = map[int32]string{
		1:   "消费-新购",
		2:   "消费-续订",
		3:   "消费-变更",
		4:   "退款-退订",
		5:   "消费-使用",
		8:   "消费-自动续订",
		9:   "调账-补偿",
		14:  "消费-服务支持计划月末扣费",
		15:  "消费-税金",
		16:  "调账-扣费",
		17:  "消费-保底差额",
		20:  "退款-变更",
		100: "退款-退订税金",
		101: "调账-补偿税金",
		102: "调账-扣费税金",
	}

	huaWeiExcelHeader = []string{"产品名称", "云服务区名称", "金额单位", "使用量类型", "使用量度量单位", "云服务类型编码",
		"云服务类型名称", "资源类型编码", "资源类型名称", "计费模式", "账单类型", "套餐内使用量", "使用量", "预留实例使用量", "币种",
		"汇率", "本期应付外币金额（元）", "本期应付人民币金额（元）"}
)

func (b *billItemSvc) exportHuaweiBillItems(kt *kit.Kit, req *bill.ExportBillItemReq,
	rate *decimal.Decimal) (any, error) {

	result, err := fetchHuaweiBillItems(kt, b, req)
	if err != nil {
		logs.Errorf("fetch huawei bill items failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	rootAccountMap, mainAccountMap, bizNameMap, err := fetchAccountBizInfo(kt, b, result)
	if err != nil {
		logs.Errorf("fetch account biz info failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, append(commonExcelHeader, huaWeiExcelHeader...))
	table, err := convertHuaweiBillItems(result, bizNameMap, mainAccountMap, rootAccountMap, rate)
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

func convertHuaweiBillItems(items []*billapi.HuaweiBillItem, bizNameMap map[int64]string,
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

		extension := item.Extension.ResFeeRecordV2
		if extension == nil {
			extension = &model.ResFeeRecordV2{}
		}

		var tmp = []string{
			string(mainAccount.Site),
			fmt.Sprintf("%d%02d", item.BillYear, item.BillMonth),
			bizNameMap[item.BkBizID],
			rootAccount.ID,
			mainAccount.ID,
			converter.PtrToVal[string](extension.RegionName),
			converter.PtrToVal[string](extension.ProductName),
			converter.PtrToVal[string](extension.Region),
			huaWeiMeasureIdMap[converter.PtrToVal[int32](extension.MeasureId)], // 金额单位。 1：元
			converter.PtrToVal[string](extension.UsageType),
			conv.ToString(converter.PtrToVal[int32](extension.UsageMeasureId)),
			converter.PtrToVal[string](extension.CloudServiceType),
			converter.PtrToVal[string](extension.CloudServiceTypeName),
			converter.PtrToVal[string](extension.ResourceType),
			converter.PtrToVal[string](extension.ResourceTypeName),
			huaWeiChargeModeMap[converter.PtrToVal[string](extension.ChargeMode)],
			huaWeiBillTypeMap[converter.PtrToVal[int32](extension.BillType)],
			conv.ToString(converter.PtrToVal[float64](extension.FreeResourceUsage)),
			conv.ToString(converter.PtrToVal[float64](extension.Usage)),
			conv.ToString(converter.PtrToVal[float64](extension.RiUsage)),
			string(item.Currency),
			rate.String(),
			item.Cost.String(),
			item.Cost.Mul(*rate).String(),
		}
		result = append(result, tmp)
	}
	return result, nil
}

func fetchHuaweiBillItems(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) (
	[]*billapi.HuaweiBillItem, error) {

	totalCount, err := fetchHuaweiBillItemCount(kt, b, req)
	if err != nil {
		logs.Errorf("fetch huawei bill item count failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	exportLimit := min(totalCount, req.ExportLimit)

	commonOpt := &databill.ItemCommonOpt{
		Vendor: enumor.HuaWei,
		Year:   req.BillYear,
		Month:  req.BillMonth,
	}
	result := make([]*billapi.HuaweiBillItem, 0, exportLimit)
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
		tmpResult, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt, billListReq)
		if err != nil {
			logs.Errorf("list huawei bill item failed: %v, rid: %s", err, kt.Rid)
			return nil, err
		}
		result = append(result, tmpResult.Details...)
	}
	return result, nil
}

func fetchHuaweiBillItemCount(kt *kit.Kit, b *billItemSvc, req *bill.ExportBillItemReq) (uint64, error) {
	countReq := &databill.BillItemListReq{
		ItemCommonOpt: &databill.ItemCommonOpt{
			Vendor: enumor.HuaWei,
			Year:   req.BillYear,
			Month:  req.BillMonth,
		},
		ListReq: &core.ListReq{Filter: req.Filter, Page: core.NewCountPage()},
	}
	details, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt, countReq)
	if err != nil {
		return 0, err
	}
	return details.Count, nil
}
