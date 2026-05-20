package utils

import (
	"context"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// SplitYAMLDocuments 简单按 --- 分割 YAML 文档
func SplitYAMLDocuments(data []byte) [][]byte {
	var docs [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' && i+2 < len(data) && data[i+1] == '-' && data[i+2] == '-' && data[i+3] == '-' {
			// 发现 ---，把前面的内容作为一个文档
			if start < i {
				docs = append(docs, data[start:i])
			}
			start = i + 4
			for start < len(data) && data[start] == '\n' {
				start++
			}
			i = start
		}
	}
	if start < len(data) {
		docs = append(docs, data[start:])
	}
	return docs
}

// ApplyK8sYaml 模拟kubectl apply -f {}.yaml
func ApplyK8sYaml(path, namespace string, c client.Client) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	docs := SplitYAMLDocuments(data)
	for _, doc := range docs {
		var objMap map[string]interface{}
		if err = yaml.Unmarshal(doc, &objMap); err != nil {
			return err
		}
		obj := &unstructured.Unstructured{Object: objMap}
		obj.SetNamespace(namespace)
		if err = c.Patch(context.TODO(), obj, client.Apply, client.FieldOwner("kserve")); err != nil {
			return err
		}
	}
	return nil
}

// DeleteK8sYaml 模拟kubectl delete -f {}.yaml
func DeleteK8sYaml(path, namespace string, c client.Client) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	docs := SplitYAMLDocuments(data)
	for _, doc := range docs {
		var objMap map[string]interface{}
		if err = yaml.Unmarshal(doc, &objMap); err != nil {
			return err
		}
		obj := &unstructured.Unstructured{Object: objMap}
		obj.SetNamespace(namespace)
		if err = c.Delete(context.TODO(), obj); err != nil {
			return err
		}
	}
	return nil
}
