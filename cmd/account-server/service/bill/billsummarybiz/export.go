package billsummarybiz

import (
	"fmt"
	"time"

	"hcm/cmd/account-server/logics/bill/export"
	"hcm/pkg/api/account-server/bill"
	"hcm/pkg/api/core"
	billproto "hcm/pkg/api/data-service/bill"
	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/criteria/constant"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/iam/meta"
	"hcm/pkg/kit"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/thirdparty/esb/cmdb"
	"hcm/pkg/tools/encode"
	"hcm/pkg/tools/slice"

	"github.com/TencentBlueKing/gopkg/conv"
)

var (
	excelHeader = []string{"运营产品ID", "运营产品名称", "已确认账单人民币（元）", "已确认账单美金（美元）",
		"当前账单人民币（元）", "当前账单美金（美元）"}
)

// ExportBizSummary export biz summary with options
func (s *service) ExportBizSummary(cts *rest.Contexts) (interface{}, error) {
	req := new(bill.BizSummaryExportReq)
	if err := cts.DecodeInto(req); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, errf.NewFromErr(errf.InvalidParameter, err)
	}

	err := s.authorizer.AuthorizeWithPerm(cts.Kit,
		meta.ResourceAttribute{Basic: &meta.Basic{Type: meta.AccountBill, Action: meta.Find}})
	if err != nil {
		return nil, err
	}

	result, err := s.fetchBizSummary(cts, req)
	if err != nil {
		logs.Errorf("fetch biz summary failed: %v, rid: %s")
		return nil, err
	}

	bkBizIDs := make([]int64, 0, len(result))
	for _, detail := range result {
		bkBizIDs = append(bkBizIDs, detail.BkBizID)
	}
	bizMap, err := s.listBiz(cts.Kit, bkBizIDs)
	if err != nil {
		logs.Errorf("list biz failed: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	data := make([][]string, 0, len(result)+1)
	data = append(data, excelHeader)
	data = append(data, toRawData(result, bizMap)...)
	buf, err := export.GenerateCSV(data)
	if err != nil {
		logs.Errorf("generate csv failed: %v, data: %v, rid: %s", err, data, cts.Kit.Rid)
		return nil, err
	}

	filename := fmt.Sprintf("%s/bill_summary_biz_%s.csv", constant.BillExportFolderPrefix,
		time.Now().Format("20060102150405"))
	base64Str, err := encode.ReaderToBase64Str(buf)
	if err != nil {
		return nil, err
	}
	if err = s.client.DataService().Global.Cos.Upload(cts.Kit,
		&cos.UploadFileReq{
			Filename:   filename,
			FileBase64: base64Str,
		}); err != nil {
		return nil, err
	}
	url, err := s.client.DataService().Global.Cos.GenerateTemporalUrl(cts.Kit, "download",
		&cos.GenerateTemporalUrlReq{
			Filename:   filename,
			TTLSeconds: 3600,
		})
	if err != nil {
		logs.Errorf("generate url failed: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	return bill.BillExportResult{DownloadURL: url.URL}, nil
}

func toRawData(details []*billproto.BillSummaryBizResult, bizMap map[int64]string) [][]string {
	result := make([][]string, 0, len(details))
	for _, detail := range details {
		row := []string{
			conv.ToString(detail.BkBizID),
			bizMap[detail.BkBizID],
			detail.CurrentMonthRMBCostSynced.String(),
			detail.CurrentMonthCostSynced.String(),
			detail.CurrentMonthRMBCost.String(),
			detail.CurrentMonthCost.String(),
		}
		result = append(result, row)
	}
	return result
}

func (s *service) fetchBizSummary(cts *rest.Contexts, req *bill.BizSummaryExportReq) (
	[]*billproto.BillSummaryBizResult, error) {

	var expression = tools.ExpressionAnd(
		tools.RuleEqual("bill_year", req.BillYear),
		tools.RuleEqual("bill_month", req.BillMonth),
	)
	if len(req.BKBizIDs) > 0 {
		var err error
		expression, err = tools.And(expression, tools.RuleIn("bk_biz_id", req.BKBizIDs))
		if err != nil {
			return nil, err
		}
	}
	details, err := s.client.DataService().Global.Bill.ListBillSummaryBiz(cts.Kit, &core.ListReq{
		Filter: expression,
		Page:   core.NewCountPage(),
	})
	if err != nil {
		return nil, err
	}

	limit := details.Count
	if req.ExportLimit <= limit {
		limit = req.ExportLimit
	}

	result := make([]*billproto.BillSummaryBizResult, 0, limit)
	page := core.DefaultMaxPageLimit
	for offset := uint64(0); offset < limit; offset = offset + uint64(core.DefaultMaxPageLimit) {
		if limit-offset < uint64(page) {
			page = uint(limit - offset)
		}
		tmpResult, err := s.client.DataService().Global.Bill.ListBillSummaryBiz(cts.Kit, &core.ListReq{
			Filter: expression,
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

func (s *service) listBiz(kt *kit.Kit, ids []int64) (map[int64]string, error) {
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
	resp, err := s.esbClient.Cmdb().SearchBusiness(kt, params)
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
