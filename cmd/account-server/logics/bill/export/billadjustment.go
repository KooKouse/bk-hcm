package export

import "hcm/pkg/logs"

// BillAdjustmentTableHeader 账单调整导出表头
var BillAdjustmentTableHeader []string

var _ Table = (*BillAdjustmentTable)(nil)

func init() {
	var err error
	BillAdjustmentTableHeader, err = BillAdjustmentTable{}.GetHeaders()
	if err != nil {
		logs.Errorf("bill adjustment table header init failed: %v", err)
	}
}

// BillAdjustmentTable 账单调整导出表头结构
type BillAdjustmentTable struct {
	UpdateTime string `header:"更新时间"`
	BillID     string `header:"调账ID"`

	BKBizName string `header:"业务"`

	MainAccountName string `header:"二级账号名称"`
	AdjustType      string `header:"调账类型"`
	Operator        string `header:"操作人"`
	Cost            string `header:"金额"`
	Currency        string `header:"币种"`
	AdjustStatus    string `header:"调账状态"`
}

// GetHeaderFields ...
func (b BillAdjustmentTable) GetHeaderFields() ([]string, error) {
	return parseHeaderFields(b)
}

// GetHeaders ...
func (b BillAdjustmentTable) GetHeaders() ([]string, error) {
	return parseHeader(b)
}
