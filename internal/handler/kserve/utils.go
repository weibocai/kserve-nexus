package kserve

import (
	"context"
	"os"

	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// 获取isvc的运行模式
func (kh *Handler) getDeploymentMode(ctx context.Context, dm string, annotations map[string]string, cm *corev1.ConfigMap) (ksvcconstants.DeploymentModeType, error) {
	if dm != "" {
		return ksvcconstants.DeploymentModeType(dm), nil
	}
	deploymentMode, ok := annotations[ksvcconstants.DeploymentMode]
	if deploymentMode == string(ksvcconstants.LegacyRawDeployment) {
		// LegacyRawDeployment is deprecated, so we treat it as Standard
		deploymentMode = string(ksvcconstants.Standard)
	}
	if deploymentMode == string(ksvcconstants.LegacyServerless) {
		// LegacyServerless is deprecated, so we treat it as Knative
		deploymentMode = string(ksvcconstants.Knative)
	}
	if ok && (deploymentMode == string(ksvcconstants.Standard) ||
		deploymentMode == string(ksvcconstants.Knative) ||
		deploymentMode == string(ksvcconstants.ModelMeshDeployment)) {
		return ksvcconstants.DeploymentModeType(deploymentMode), nil
	}
	if cm == nil {
		if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: ksvcconstants.KServeNamespace, Name: ksvcconstants.InferenceServiceConfigMapName}, cm); err != nil {
			return "", err
		}
	}

	deployConfig, err := ksvcv1beta1.NewDeployConfig(cm)
	if err != nil {
		return "", err
	}
	// Finally, if an InferenceService is being created and does not explicitly specify a DeploymentMode
	return ksvcconstants.DeploymentModeType(deployConfig.DefaultDeploymentMode), nil
}

// 简单按 --- 分割 YAML 文档
func splitYAMLDocuments(data []byte) [][]byte {
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

// 模拟kubectl apply -f {}.yaml
func applyK8sYaml(path, namespace string, c client.Client) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	docs := splitYAMLDocuments(data)
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

// 模拟kubectl delete -f {}.yaml
func deleteK8sYaml(path, namespace string, c client.Client) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	docs := splitYAMLDocuments(data)
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
