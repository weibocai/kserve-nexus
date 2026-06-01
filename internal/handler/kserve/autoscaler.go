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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/pkg/log"
	"github.com/kserve-nexus/pkg/utils"
)

// Autoscaler 自动伸缩组件
type Autoscaler struct {
	Namespace           string                                 // 命名空间
	Hpa                 *autoscalingv2.HorizontalPodAutoscaler // hpa
	HpaStatus           GraphNodeStatus                        // hpa 状态
	HpaName             string                                 // hpa name
	Keda                *kedav1alpha1.ScaledObject             // keda
	KedaStatus          GraphNodeStatus                        // keda 状态
	KedaName            string                                 // keda name
	AutoscalerClassType ksvcconstants.AutoscalerClassType      // 自动伸缩类型
}

// ToGraphNode 转换成图节点
func (h *Autoscaler) ToGraphNode(nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) (*GraphNode, *GraphNode) {
	var head, tail *GraphNode
	switch h.AutoscalerClassType {
	case ksvcconstants.AutoscalerClassKeda:
		head = nodes.AddNodes(h.KedaName, h.Namespace, utils.GetCrdKey("keda"), h.KedaStatus, belong, parent...)
		fallthrough
	case ksvcconstants.AutoscalerClassHPA, ksvcconstants.AutoscalerClassExternal, ksvcconstants.AutoscalerClassNone:
		if head != nil {
			tail = nodes.AddNodes(h.HpaName, h.Namespace, utils.GetCrdKey("hpa"), h.HpaStatus, belong, head)
		} else {
			head = nodes.AddNodes(h.HpaName, h.Namespace, utils.GetCrdKey("hpa"), h.HpaStatus, belong, parent...)
			tail = head
		}
	default:
	}
	return head, tail
}

// SetKedaStatus 设置keda的状态
func (h *Autoscaler) SetKedaStatus() {
	for _, c := range h.Keda.Status.Conditions {
		if (c.Type == kedav1alpha1.ConditionActive || c.Type == kedav1alpha1.ConditionReady) && c.Status != metav1.ConditionTrue {
			h.KedaStatus = GraphNodeStatusFalse
			return
		}
	}
	h.KedaStatus = GraphNodeStatusTrue
}

// SetHpaStatus 设置hpa的状态
func (h *Autoscaler) SetHpaStatus() {
	for _, c := range h.Hpa.Status.Conditions {
		if (c.Type == autoscalingv2.ScalingActive || c.Type == autoscalingv2.AbleToScale) && c.Status != corev1.ConditionTrue {
			h.HpaStatus = GraphNodeStatusFalse
			return
		}
	}
	h.HpaStatus = GraphNodeStatusTrue
	return
}

// NewAutoscaler：新建 ksvc 自动伸缩组件
func NewAutoscaler(ctx context.Context, kc client.Client, ac ksvcconstants.AutoscalerClassType, isvcName, name, namespace string) *Autoscaler {
	autoscaler := Autoscaler{
		Hpa: nil, HpaStatus: GraphNodeStatusFalse, HpaName: name, Keda: nil, KedaStatus: GraphNodeStatusFalse, KedaName: name,
		AutoscalerClassType: ac, Namespace: namespace,
	}

	switch ac {
	case ksvcconstants.AutoscalerClassKeda:
		keda := &kedav1alpha1.ScaledObject{}
		if err := kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, keda); err != nil {
			log.Logger.Error(err, "推理服务hpa keda 获取失败", "namespace", namespace, "name", isvcName)
		} else {
			autoscaler.Keda = keda
		}
		autoscaler.SetKedaStatus()
		name = "keda-hpa-" + name
		fallthrough
	case ksvcconstants.AutoscalerClassHPA, ksvcconstants.AutoscalerClassExternal, ksvcconstants.AutoscalerClassNone:
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, hpa); err != nil {
			log.Logger.Error(err, "推理服务hpa 获取失败", "namespace", namespace, "name", isvcName)
		} else {
			autoscaler.Hpa = hpa
		}
		autoscaler.SetHpaStatus()
	case "":

	default:
		log.Logger.Error(fmt.Errorf("unknown autoscaler class type: %v", ac), "推理服务hpa keda 获取失败", "namespace", namespace, "name", isvcName)

	}
	return &autoscaler
}
