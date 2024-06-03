package lblogic

import (
	"fmt"
	"hcm/pkg/criteria/enumor"
	"strconv"
	"strings"
)

var (
	supportedProtocols = map[enumor.ProtocolType]struct{}{
		enumor.TcpProtocol:   {},
		enumor.UdpProtocol:   {},
		enumor.HttpProtocol:  {},
		enumor.HttpsProtocol: {},
	}

	supportRSTypes = map[enumor.InstType]struct{}{
		enumor.CvmInstType: {},
		enumor.EniInstType: {},
	}
)

// ListenerRawInput 用于解析Excel中每一行的输入数据
type ListenerRawInput struct {
	RuleName       string
	Protocol       string
	VIPs           []string
	VPorts         []int
	Domain         string // 域名
	URLPath        string // URL路径
	RSIPs          []string
	RSPorts        []int
	Weight         []int
	Scheduler      string // 均衡方式
	SessionExpired int64  // 会话保持时间，单位秒
	InstType       string // 后端类型 CVM、ENI
	ServerCert     string // ref: pkg/api/core/cloud/load-balancer/tcloud.go:188
	ClientCert     string // 客户端证书
	HealthCheck    bool   // 是否开启健康检查
}

// Validate validates the input data
func (l *ListenerRawInput) Validate() error {
	err := l.validateRS()
	if err != nil {
		return err
	}

	if err = l.validateProtocol(); err != nil {
		return err
	}

	if err = l.validateSessionExpired(); err != nil {
		return err
	}

	return nil
}

func (l *ListenerRawInput) validateSessionExpired() error {
	if l.SessionExpired == 0 || (l.SessionExpired >= 30 && l.SessionExpired <= 3600) {
		return nil
	}
	return fmt.Errorf("session expired should be 0 or between 30 and 3600")
}

func (l *ListenerRawInput) validateProtocol() error {
	if _, ok := supportedProtocols[enumor.ProtocolType(l.Protocol)]; !ok {
		return fmt.Errorf("unsupported protocol: %s", l.Protocol)
	}

	// 4层负载均衡和7层的校验的策略会有区别

	return nil
}

func (l *ListenerRawInput) validateInstType() error {
	return nil
}

func (l *ListenerRawInput) validateWieght() error {
	return nil
}

func (l *ListenerRawInput) validateScheduler() error {
	//WRR、LEAST_CONN、IP_HASH
	if l.Scheduler != "wrr" && l.Scheduler != "rr" {

	}

	return nil
}

func (l *ListenerRawInput) validateRS() error {
	if len(l.RSPorts) > 1 && len(l.RSPorts) != len(l.RSIPs) {
		return fmt.Errorf("the number of RSPorts and RSIPs should be equal or 1")
	}

	if len(l.Weight) > 1 && len(l.Weight) != len(l.RSIPs) {
		return fmt.Errorf("the number of Weight and RSIPs should be equal or 1")
	}

	if len(l.RSIPs) == 1 {
		return nil
	}

	/** 数据补全
	input: RSIPs: [1.1.1.1 2.2.2.2] RSPorts: [80] Weight: [1 1]
	output: RSIPs: [1.1.1.1 2.2.2.2] RSPorts: [80 80] Weight: [1 1]

	input: RSIPs: [1.1.1.1 2.2.2.2] RSPorts: [80 80] Weight: [1]
	output: RSIPs: [1.1.1.1 2.2.2.2] RSPorts: [80 80] Weight: [1 1]
	*/
	for len(l.RSPorts) < len(l.RSIPs) {
		l.RSPorts = append(l.RSPorts, l.RSPorts[0])
	}

	for len(l.Weight) < len(l.RSIPs) {
		l.Weight = append(l.Weight, l.Weight[0])
	}

	return nil
}

// parseIPs 解析使用回车分隔的IP地址字符串
func parseIPs(ipStr string) []string {
	return strings.Split(ipStr, "\n")
}

// parsePorts 解析端口字符串，支持换行符分隔和端口范围格式
func parsePorts(portStr, sep string) ([]int, error) {
	if strings.HasPrefix(portStr, "[") && strings.HasSuffix(portStr, "]") {
		portsStr := portStr[1 : len(portStr)-1] // 移除括号
		portsStr = strings.TrimSpace(portsStr)
		// 分割端口范围
		ports := strings.Split(portsStr, ",")
		if len(ports) != 2 {
			return nil, fmt.Errorf("invalid port range format: %s", portStr)
		}
		startPort, err := strconv.Atoi(strings.TrimSpace(ports[0]))
		if err != nil {
			return nil, err
		}
		endPort, err := strconv.Atoi(strings.TrimSpace(ports[1]))
		if err != nil {
			return nil, err
		}
		// 生成端口范围内的所有端口
		var portsSlice []int
		for i := startPort; i <= endPort; i++ {
			portsSlice = append(portsSlice, i)
		}
		return portsSlice, nil
	} else {
		// 换行符分隔的多个端口
		return parseIntSlice(portStr, sep)
	}
}

// parseIntSlice 用于将分隔符分隔的字符串转换为整数切片
func parseIntSlice(s, sep string) ([]int, error) {
	parts := strings.Split(s, sep)
	ints := make([]int, 0, len(parts))
	for _, part := range parts {
		port, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		ints = append(ints, port)
	}
	return ints, nil
}

// parseHealthCheck 解析健康检查字段，将"是"转换为true，"否"转换为false
func parseHealthCheck(healthCheckStr string) (bool, error) {
	switch strings.TrimSpace(healthCheckStr) {
	case "是":
		return true, nil
	case "否":
		return false, nil
	default:
		return false, fmt.Errorf("invalid value for health check: %s, should be '是' or '否'", healthCheckStr)
	}
}

// ParseListener parse excel row data to ListenerRawInput struct
func ParseListener(row []string) (*ListenerRawInput, error) {
	rc := new(ListenerRawInput)
	var err error
	rc.RuleName = row[0]
	rc.Protocol = row[1]
	rc.VIPs = parseIPs(row[2])
	rc.VPorts, err = parsePorts(row[3], ";")
	if err != nil {
		return nil, fmt.Errorf("Error parsing VPorts: %v\n", err)
	}
	rc.Domain = row[4]
	rc.URLPath = row[5]

	rc.RSIPs = parseIPs(row[6])

	rc.RSPorts, err = parsePorts(row[7], "\n")
	if err != nil {
		return nil, fmt.Errorf("Error parsing RSPORTs: %v\n", err)
	}

	rc.Weight, err = parseIntSlice(row[8], "\n")
	if err != nil {
		return nil, fmt.Errorf("Error parsing Weight: %v\n", err)

	}
	rc.Scheduler = row[9]
	rc.SessionExpired, err = strconv.ParseInt(row[10], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Error parsing Sticky: %v\n", err)
	}
	rc.InstType = row[11]
	rc.ServerCert = row[12]
	rc.ClientCert = row[13]
	rc.HealthCheck, err = parseHealthCheck(row[14])
	if err != nil {
		return nil, fmt.Errorf("Error parsing HealthCheck: %v\n", err)
	}
	return rc, nil
}
