package export

import "hcm/pkg/logs"

// HuaweiBillItemHeaders is the headers of GCP bill item.
var HuaweiBillItemHeaders []string

func init() {
	var err error
	HuaweiBillItemHeaders, err = HuaweiBillItemTable{}.GetHeaders()
	if err != nil {
		logs.Errorf("GetHuaweiHeader failed: %v", err)
	}
}

var _ Table = (*HuaweiBillItemTable)(nil)

// HuaweiBillItemTable huawei账单导出表结构
type HuaweiBillItemTable struct {
	Site        string `header:"站点类型"`
	AccountDate string `header:"核算年月"`

	BizName string `header:"业务名称"`

	RootAccountName string `header:"一级账号名称"`
	MainAccountName string `header:"二级账号名称"`
	Region          string `header:"地域"`

	ProductName          string `header:"产品名称"`
	RegionName           string `header:"云服务区名称"`
	MeasureID            string `header:"金额单位"`
	UsageType            string `header:"使用量类型"`
	UsageMeasureID       string `header:"使用量度量单位"`
	CloudServiceType     string `header:"云服务类型编码"`
	CloudServiceTypeName string `header:"云服务类型名称"`
	ResourceType         string `header:"资源类型编码"`
	ResourceTypeName     string `header:"资源类型名称"`
	ChargeMode           string `header:"计费模式"`
	BillType             string `header:"账单类型"`
	FreeResourceUsage    string `header:"套餐内使用量"`
	Usage                string `header:"使用量"`
	RiUsage              string `header:"预留实例使用量"`
	Currency             string `header:"币种"`
	ExchangeRate         string `header:"汇率"`
	Cost                 string `header:"本期应付外币金额（元）"`
	CostRMB              string `header:"本期应付人民币金额（元）"`
}

// GetHeaders ...
func (h HuaweiBillItemTable) GetHeaders() ([]string, error) {
	return parseHeader(h)
}

// GetHeaderFields ...
func (h HuaweiBillItemTable) GetHeaderValues() ([]string, error) {
	return parseHeaderFields(h)
}
