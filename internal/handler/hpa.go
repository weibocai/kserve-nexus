package kserve

import (
	"context"
	"fmt"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kserve-nexus/pkg/log"
)

func (kh *KserveHandler) getKedaStatus(keda *kedav1alpha1.ScaledObject) isvcGraphNodeStatus {
	for _, c := range keda.Status.Conditions {
		if (c.Type == kedav1alpha1.ConditionActive || c.Type == kedav1alpha1.ConditionReady) && c.Status != metav1.ConditionTrue {
			return isvcGraphNodeStatusFalse
		}
	}
	return isvcGraphNodeStatusTrue
}

func (kh *KserveHandler) getHpaStatus(hpa *autoscalingv2.HorizontalPodAutoscaler) isvcGraphNodeStatus {
	for _, c := range hpa.Status.Conditions {
		if (c.Type == autoscalingv2.ScalingActive || c.Type == autoscalingv2.AbleToScale) && c.Status != corev1.ConditionTrue {
			return isvcGraphNodeStatusFalse
		}
	}
	return isvcGraphNodeStatusTrue
}

func (kh *KserveHandler) getAutoscaler(ctx context.Context, ac ksvcconstants.AutoscalerClassType, isvcName, name, namespace string) (*autoscalingv2.HorizontalPodAutoscaler, isvcGraphNodeStatus, *kedav1alpha1.ScaledObject, isvcGraphNodeStatus) {
	var keda *kedav1alpha1.ScaledObject
	var hpa *autoscalingv2.HorizontalPodAutoscaler
	kedaStatus, hpaStatus := isvcGraphNodeStatusFalse, isvcGraphNodeStatusFalse
	switch ac {
	case ksvcconstants.AutoscalerClassKeda:
		keda = &kedav1alpha1.ScaledObject{}
		if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, keda); err != nil {
			log.Logger.Error(err, "推理服务hpa keda 获取失败", "namespace", namespace, "name", isvcName)
		} else {
			kedaStatus = kh.getKedaStatus(keda)
		}
		name = "keda-hpa-" + name
		fallthrough
	case ksvcconstants.AutoscalerClassHPA, ksvcconstants.AutoscalerClassExternal, ksvcconstants.AutoscalerClassNone:
		hpa = &autoscalingv2.HorizontalPodAutoscaler{}
		if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, hpa); err != nil {
			log.Logger.Error(err, "推理服务hpa 获取失败", "namespace", namespace, "name", isvcName)
		} else {
			hpaStatus = kh.getHpaStatus(hpa)
		}
	default:
		log.Logger.Error(fmt.Errorf("unknown autoscaler class type: %v", ac), "推理服务hpa keda 获取失败", "namespace", namespace, "name", isvcName)
		return nil, isvcGraphNodeStatusFalse, nil, isvcGraphNodeStatusFalse
	}
	return hpa, hpaStatus, keda, kedaStatus
}
