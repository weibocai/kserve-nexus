package kserve

import (
	"context"

	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
