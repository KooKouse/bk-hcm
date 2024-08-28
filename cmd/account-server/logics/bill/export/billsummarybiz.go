package export

import "hcm/pkg/logs"

// BillSummaryBizTableHeader 账单调整导出表头
var BillSummaryBizTableHeader []string

var _ Table = (*BillSummaryBizTable)(nil)

func init() {
	var err error
	BillSummaryBizTableHeader, err = BillSummaryBizTable{}.GetHeaders()
	if err != nil {
		logs.Errorf("bill adjustment table header init failed: %v", err)
	}
}

// BillSummaryBizTable 账单调整导出表头结构
type BillSummaryBizTable struct {
	BkBizID                   string `header:"业务ID"`
	BkBizName                 string `header:"业务"`
	CurrentMonthRMBCostSynced string `header:"已确认账单人民币（元）"`
	CurrentMonthCostSynced    string `header:"已确认账单美金（美元）"`
	CurrentMonthRMBCost       string `header:"当前账单人民币（元）"`
	CurrentMonthCost          string `header:"当前账单美金（美元）"`
}

// GetHeaderValues ...
func (b BillSummaryBizTable) GetHeaderValues() ([]string, error) {
	return parseHeaderFields(b)
}

// GetHeaders ...
func (b BillSummaryBizTable) GetHeaders() ([]string, error) {
	return parseHeader(b)
}
