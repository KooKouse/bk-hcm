package export

import (
	"reflect"

	"github.com/TencentBlueKing/gopkg/conv"
)

// Table excel表结构抽象
type Table interface {
	// GetHeaderValues 根据表头字段顺序解析数据
	GetHeaderValues() ([]string, error)
	// GetHeaders 获取表头列
	GetHeaders() ([]string, error)
}

func parseHeaderFields(obj interface{}) ([]string, error) {
	rt := reflect.TypeOf(obj)
	rv := reflect.ValueOf(obj)

	var headers []string
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.Tag.Get("header") != "" && !field.Anonymous {
			value := rv.Field(i)
			headers = append(headers, conv.ToString(value.Interface()))
		}
	}
	return headers, nil
}

func parseHeader(obj interface{}) ([]string, error) {
	rt := reflect.TypeOf(obj)

	var headers []string
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get("header")
		if tag != "" && !field.Anonymous {
			headers = append(headers, tag)
		}
	}
	return headers, nil
}
