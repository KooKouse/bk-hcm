/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package billitem

import (
	"bytes"
	"fmt"
	"time"

	"hcm/cmd/account-server/logics/bill/export"
	accountset "hcm/pkg/api/core/account-set"
	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/thirdparty/esb/cmdb"
	"hcm/pkg/tools/slice"

	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	billapi "hcm/pkg/api/core/bill"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"

	"github.com/shopspring/decimal"
)

var (
	commonExcelHeader = []string{"站点类型", "核算年月", "业务名称", "一级帐号名称", "二级帐号名称", "地域"}

	gcpExcelHeader = []string{"Region位置", "项目ID", "项目名称", "服务分类", "服务分类名称", "Sku名称", "外币类型",
		"用量单位", "用量", "外币成本(元)", "汇率", "人民币成本(元)"}

	azureExcelHeader = []string{"区域", "地区编码", "核算年月", "业务名称",
		"账号邮箱", "子账号名称", "服务一级类别名称", "服务二级类别名称", "服务三级类别名称", "产品名称", "资源类别",
		"计量地区", "资源地区编码", "单位", "用量", "折后税前成本（外币）", "币种", "汇率", "RMB成本（元）"}

	huaWeiExcelHeader = []string{"产品名称", "云服务区名称", "金额单位", "使用量类型", "使用量度量单位", "云服务类型编码",
		"云服务类型名称", "资源类型编码", "资源类型名称", "计费模式", "账单类型", "套餐内使用量", "使用量", "预留实例使用量", "币种",
		"汇率", "本期应付外币金额（元）", "本期应付人民币金额（元）"}

	awsExcelHeader = []string{"地区名称", "发票ID", "账单实体", "产品代号", "服务组", "产品名称", "API操作", "产品规格",
		"实例类型", "资源ID", "计费方式", "计费类型", "计费说明", "用量", "单位", "折扣前成本（外币）", "外币种类",
		"人民币成本（元）", "汇率"}
)

// ExportBillItems 导出账单明细
func (b *billItemSvc) ExportBillItems(cts *rest.Contexts) (any, error) {
	vendor := enumor.Vendor(cts.PathParameter("vendor").String())
	if len(vendor) == 0 {
		return nil, errf.New(errf.InvalidParameter, "vendor is required")
	}

	req := new(bill.ExportBillItemReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err := b.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	var mergedFilter = tools.ExpressionAnd(
		tools.RuleEqual("vendor", vendor),
		tools.RuleEqual("bill_year", req.BillYear),
		tools.RuleEqual("bill_month", req.BillMonth),
	)

	if req.Filter != nil {
		mergedFilter, err = tools.And(mergedFilter, req.Filter)
		if err != nil {
			logs.Errorf("fail merge filter for exporting bill items, err: %v, req: %+v, rid: %s", err, req, cts.Kit.Rid)
			return nil, err
		}
	}

	// 获取汇率
	result, err := b.client.DataService().Global.Bill.ListExchangeRate(cts.Kit, &core.ListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("from_currency", enumor.CurrencyUSD),
			tools.RuleEqual("to_currency", enumor.CurrencyRMB),
			tools.RuleEqual("year", req.BillYear),
			tools.RuleEqual("month", req.BillMonth),
		),
		Page: &core.BasePage{
			Start: 0,
			Limit: 1,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get exchange rate from %s to %s in %d-%d failed, err %s",
			enumor.CurrencyUSD, enumor.CurrencyRMB, req.BillYear, req.BillMonth, err.Error())
	}
	if len(result.Details) == 0 {
		return nil, fmt.Errorf("get no exchange rate from %s to %s in %d-%d, rid %s",
			enumor.CurrencyUSD, enumor.CurrencyRMB, req.BillYear, req.BillMonth, cts.Kit.Rid)
	}
	if result.Details[0].ExchangeRate == nil {
		return nil, fmt.Errorf("get exchange rate is nil, from %s to %s in %d-%d, rid %s",
			enumor.CurrencyUSD, enumor.CurrencyRMB, req.BillYear, req.BillMonth, cts.Kit.Rid)
	}

	switch vendor {
	case enumor.HuaWei:
		return exportHuaweiBillItems(cts.Kit, b, mergedFilter, req.ExportLimit, result.Details[0].ExchangeRate)
	case enumor.Azure:
		return exportAzureBillItems(cts.Kit, b, mergedFilter, req.ExportLimit, result.Details[0].ExchangeRate)
	case enumor.Gcp:
		return exportGcpBillItems(cts.Kit, b, mergedFilter, req.ExportLimit, result.Details[0].ExchangeRate)
	case enumor.Aws:
		return exportAwsBillItems(cts.Kit, b, mergedFilter, req.ExportLimit, result.Details[0].ExchangeRate)
	case enumor.Zenlayer:
		return exportZenlayerBillItems(cts.Kit, b, mergedFilter, req.ExportLimit, result.Details[0].ExchangeRate)
	default:
		return nil, fmt.Errorf("unsupport %s vendor", vendor)
	}
}

func exportZenlayerBillItems(kt *kit.Kit, b *billItemSvc, filter *filter.Expression,
	requireCount uint64, rate *decimal.Decimal) (any, error) {

	details, err := b.client.DataService().Zenlayer.Bill.ListBillItem(kt,
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

	result := make([]*billapi.ZenlayerBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := b.client.DataService().Zenlayer.Bill.ListBillItem(kt,
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

	// var azureExcelHeader = []string{"区域", "地区编码", "核算年月",  "事业群",
	//"业务部门", "规划产品", "运营产品", "账号邮箱", "子账号名称", "服务一级类别名称",
	//"服务二级类别名称", "服务三级类别名称", "产品名称", "资源类别", "计量地区",
	//"资源地区编码", "单位", "用量", "折后税前成本（外币）", "币种", "汇率",
	//"RMB成本（元）"}
	data := make([][]interface{}, 0, len(result)+1)
	//data = append(data, azureExcelHeader)
	// TODO parse data to excel format
	//for _, item := range result {
	//	tmp := []interface{}{
	//		item.Region,
	//		item.RegionCode,
	//		item.,
	//	}
	//}

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

var (
	//item.Extension.MeasureId, // 金额单位。 1：元
	huaWeiMeasureIdMap = map[int32]string{
		1: "元",
	}

	// item.Extension.ChargeMode, // 计费模式。 1：包年/包月3：按需10：预留实例
	huaWeiChargeModeMap = map[string]string{
		"1":  "包年/包月",
		"3":  "按需",
		"10": "预留实例",
	}

	//	item.Extension.BillType,
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
)

func exportAzureBillItems(kt *kit.Kit, b *billItemSvc, filter *filter.Expression,
	requireCount uint64, rate *decimal.Decimal) (any, error) {

	details, err := b.client.DataService().Azure.Bill.ListBillItem(kt,
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

	result := make([]*billapi.AzureBillItem, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := b.client.DataService().Azure.Bill.ListBillItem(kt,
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

	data := make([][]interface{}, 0, len(result)+1)
	//data = append(data, azureExcelHeader)
	// TODO parse data to excel format
	//for _, item := range result {
	//	tmp := []interface{}{
	//		item.Region,
	//		item.RegionCode,
	//		item.,
	//	}
	//}

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

func uploadFileAndReturnUrl(kt *kit.Kit, b *billItemSvc, buf *bytes.Buffer) (string, error) {
	filename := fmt.Sprintf("%s/bill_item_%s.csv", constant.BillExportFolderPrefix,
		time.Now().Format("20060102150405"))
	// generate filename
	if err := b.client.DataService().Global.Cos.Upload(kt, filename, buf); err != nil {
		return "", err
	}

	result, err := b.client.DataService().Global.Cos.GenerateTemporalUrl(kt, "download",
		&cos.GenerateTemporalUrlReq{
			Filename:   filename,
			TTLSeconds: 3600,
		})
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func listMainAccount(kt *kit.Kit, b *billItemSvc, ids []string) (map[string]*accountset.BaseMainAccount, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ids = slice.Unique(ids)
	expression, err := tools.And(
		tools.RuleIn("id", ids),
	)
	if err != nil {
		return nil, err
	}

	details, err := b.client.DataService().Global.MainAccount.List(kt, &core.ListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	})
	if err != nil {
		return nil, err
	}
	total := details.Count

	result := make(map[string]*accountset.BaseMainAccount, total)
	for offset := uint64(0); offset < total; offset = offset + uint64(core.DefaultMaxPageLimit) {
		tmpResult, err := b.client.DataService().Global.MainAccount.List(kt, &core.ListReq{
			Filter: expression,
			Page: &core.BasePage{
				Start: uint32(offset),
				Limit: core.DefaultMaxPageLimit,
			},
		})
		if err != nil {
			return nil, err
		}
		for _, item := range tmpResult.Details {
			result[item.ID] = item
		}
	}

	return result, nil
}

func listRootAccount(kt *kit.Kit, b *billItemSvc, ids []string) (map[string]*accountset.BaseRootAccount, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ids = slice.Unique(ids)
	expression, err := tools.And(
		tools.RuleIn("id", ids),
	)
	if err != nil {
		return nil, err
	}

	details, err := b.client.DataService().Global.RootAccount.List(kt, &core.ListWithoutFieldReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	})
	if err != nil {
		return nil, err
	}
	total := details.Count

	result := make(map[string]*accountset.BaseRootAccount, total)
	for offset := uint64(0); offset < total; offset = offset + uint64(core.DefaultMaxPageLimit) {
		tmpResult, err := b.client.DataService().Global.RootAccount.List(kt,
			&core.ListWithoutFieldReq{
				Filter: expression,
				Page: &core.BasePage{
					Start: uint32(offset),
					Limit: core.DefaultMaxPageLimit,
				},
			})
		if err != nil {
			return nil, err
		}
		for _, item := range tmpResult.Details {
			result[item.ID] = item
		}
	}

	return result, nil
}

func listBiz(kt *kit.Kit, b *billItemSvc, ids []int64) (map[int64]string, error) {
	expression := &cmdb.QueryFilter{
		Rule: &cmdb.CombinedRule{
			Condition: "AND",
			Rules: []cmdb.Rule{
				&cmdb.AtomRule{
					Field:    "bk_biz_id",
					Operator: "in",
					Value:    slice.Unique(ids),
				},
			},
		},
	}
	params := &cmdb.SearchBizParams{
		BizPropertyFilter: expression,
		Fields:            []string{"bk_biz_id", "bk_biz_name"},
	}
	resp, err := b.esbClient.Cmdb().SearchBusiness(kt, params)
	if err != nil {
		return nil, fmt.Errorf("call cmdb search business api failed, err: %v", err)
	}

	infos := resp.Info
	data := make(map[int64]string, len(infos))
	for _, biz := range infos {
		data[biz.BizID] = biz.BizName
	}

	return data, nil
}
