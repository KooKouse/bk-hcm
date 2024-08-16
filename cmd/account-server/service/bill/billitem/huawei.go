/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
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
	"fmt"

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
)

func (b *billItemSvc) exportHuaweiBillItems(kt *kit.Kit, req *bill.ExportBillItemReq,
	rate *decimal.Decimal) (any, error) {

	rootAccountMap, mainAccountMap, bizNameMap, err := b.fetchAccountBizInfo(kt, enumor.HuaWei)
	if err != nil {
		logs.Errorf("prepare related data failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	buff, writer := export.NewCsvWriter()
	if err = writer.Write(getHuaweiHeader()); err != nil {
		logs.Errorf("csv write header failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	convFunc := func(items []*billapi.HuaweiBillItem) error {
		if len(items) == 0 {
			return nil
		}
		table, err := convertHuaweiBillItems(items, bizNameMap, mainAccountMap, rootAccountMap, rate)
		if err != nil {
			logs.Errorf("convert to raw data error: %s, rid: %s", err, kt.Rid)
			return err
		}
		err = writer.WriteAll(table)
		if err != nil {
			logs.Errorf("csv write data failed: %v, rid: %s", err, kt.Rid)
			return err
		}
		return nil
	}
	err = b.fetchHuaweiBillItems(kt, req, convFunc)
	if err != nil {
		logs.Errorf("fetch huawei bill items failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	url, err := b.uploadFileAndReturnUrl(kt, buff)
	if err != nil {
		logs.Errorf("upload file failed: %v, rid: %s", err, kt.Rid)
		return nil, err
	}
	return bill.BillExportResult{DownloadURL: url}, nil
}

func getHuaweiHeader() []string {
	huaWeiExcelHeader := []string{"产品名称", "云服务区名称", "金额单位", "使用量类型", "使用量度量单位", "云服务类型编码",
		"云服务类型名称", "资源类型编码", "资源类型名称", "计费模式", "账单类型", "套餐内使用量", "使用量", "预留实例使用量", "币种",
		"汇率", "本期应付外币金额（元）", "本期应付人民币金额（元）"}
	return append(commonGetHeader(), huaWeiExcelHeader...)
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
		bizName, ok := bizNameMap[item.BkBizID]
		if !ok {
			return nil, fmt.Errorf("bizID(%d) not found", item.BkBizID)
		}

		extension := item.Extension.ResFeeRecordV2
		if extension == nil {
			extension = &model.ResFeeRecordV2{}
		}

		var tmp = []string{
			string(mainAccount.Site),
			fmt.Sprintf("%d%02d", item.BillYear, item.BillMonth),
			bizName,
			rootAccount.Name,
			mainAccount.Name,
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

func (b *billItemSvc) fetchHuaweiBillItems(kt *kit.Kit, req *bill.ExportBillItemReq,
	convertFunc func([]*billapi.HuaweiBillItem) error) error {

	totalCount, err := b.fetchHuaweiBillItemCount(kt, req)
	if err != nil {
		logs.Errorf("fetch huawei bill item count failed: %v, rid: %s", err, kt.Rid)
		return err
	}
	exportLimit := min(totalCount, req.ExportLimit)

	commonOpt := &databill.ItemCommonOpt{
		Vendor: enumor.HuaWei,
		Year:   req.BillYear,
		Month:  req.BillMonth,
	}
	lastID := ""
	for offset := uint64(0); offset < exportLimit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		left := exportLimit - offset
		expr := req.Filter
		if len(lastID) > 0 {
			expr, err = tools.And(expr, tools.RuleGreaterThan("id", lastID))
			if err != nil {
				logs.Errorf("build filter failed: %v, rid: %s", err, kt.Rid)
				return err
			}
		}
		billListReq := &databill.BillItemListReq{
			ItemCommonOpt: commonOpt,
			ListReq: &core.ListReq{
				Filter: expr,
				Page: &core.BasePage{
					Start: uint32(offset),
					Limit: min(uint(left), core.DefaultMaxPageLimit),
					Sort:  "id",
					Order: core.Ascending,
				},
			},
		}
		result, err := b.client.DataService().HuaWei.Bill.ListBillItem(kt, billListReq)
		if err != nil {
			logs.Errorf("list huawei bill item failed: %v, rid: %s", err, kt.Rid)
			return err
		}
		if len(result.Details) == 0 {
			continue
		}
		if err = convertFunc(result.Details); err != nil {
			logs.Errorf("convert huawei bill item failed: %v, rid: %s", err, kt.Rid)
			return err
		}
		lastID = result.Details[len(result.Details)-1].ID
	}
	return nil
}

func (b *billItemSvc) fetchHuaweiBillItemCount(kt *kit.Kit, req *bill.ExportBillItemReq) (uint64, error) {
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
