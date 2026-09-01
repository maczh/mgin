package s3

import "gopkg.in/yaml.v3"

// unmarshalYAML 解析 go.s3 节点传入的 yaml 字节到目标结构
func unmarshalYAML(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
