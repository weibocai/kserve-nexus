package kserve

import (
	"context"

	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/internal/handler"
	"github.com/kserve-nexus/pkg/log"
)

type SvcDepPod struct {
	Svc    *corev1.Service
	Deploy []*appsv1.Deployment
	Pod    map[string]*corev1.PodList
}

// getSvcDep 获取 service、deployment、pods
func GetSvcDep(ctx context.Context, kc client.Client, name, namespace string) *SvcDepPod {
	sdp := SvcDepPod{
		nil, make([]*appsv1.Deployment, 0), make(map[string]*corev1.PodList),
	}

	svc := &corev1.Service{}
	if err := kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, svc); err != nil {
		log.Logger.Error(err, "获取 service 失败", "name", name, "namespace", namespace)
		return &sdp
	}
	sdp.Svc = svc
	dep := &appsv1.DeploymentList{}
	if err := kc.List(ctx, dep, client.InNamespace(namespace), client.MatchingLabels(svc.Spec.Selector)); err != nil {
		log.Logger.Error(err, "获取 deployment 失败", "name", name, "namespace", namespace)
		return &sdp
	}
	for i := range dep.Items {
		sdp.Deploy = append(sdp.Deploy, &dep.Items[i])
		pods := &corev1.PodList{}
		ls, err := metav1.LabelSelectorAsSelector(dep.Items[i].Spec.Selector)
		if err != nil {
			log.Logger.Error(err, "获取 pods 失败", "name", name, "namespace", namespace)
			continue
		}
		if err = kc.List(ctx, pods, &client.ListOptions{Namespace: namespace}, &client.MatchingLabelsSelector{Selector: ls}); err != nil {
			log.Logger.Error(err, "获取 pods 失败", "name", name, "namespace", namespace)
			continue
		}
		sdp.Pod[dep.Items[i].Name] = pods
	}
	return &sdp
}

func GetServiceStatus(svc *corev1.Service) GraphNodeStatus {
	if svc == nil {
		return GraphNodeStatusFalse
	}
	return GraphNodeStatusTrue
}

func GetPodStatus(pod *corev1.Pod) GraphNodeStatus {
	podReady := GraphNodeStatusFalse
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			podReady = GraphNodeStatusTrue
			break
		}
	}
	return podReady
}

func GetDepStatus(dep *appsv1.Deployment) GraphNodeStatus {
	if dep != nil && dep.Status.AvailableReplicas == *dep.Spec.Replicas {
		return GraphNodeStatusTrue
	}
	return GraphNodeStatusFalse
}

// service、develop、pod关系
func (kh *Handler) getSvcDepShow(ctx context.Context, isvcName, name, namespace string, parent *GraphNode, ac ksvcconstants.AutoscalerClassType, nodes GraphNodeMap) *GraphNode {
	var hpaObject *GraphNode = nil
	if ac != "" {
		_, hpaStatus, _, kedaStatus := kh.getAutoscaler(ctx, ac, isvcName, name, namespace)
		hpaObject = nodes.AddNodes(name, namespace, handler.GetCrdKey("hpa"), hpaStatus, nil)
		if ac == ksvcconstants.AutoscalerClassKeda {
			kedaObject := nodes.AddNodes(name, namespace, "ScaledObject", kedaStatus, nil, parent)
			hpaObject.AddParent(kedaObject)
		} else {
			hpaObject.AddParent(parent)
		}
	}

	sdp := GetSvcDep(ctx, kh.kc, name, namespace)
	svcObject := nodes.AddNodes(name, namespace, handler.GetCrdKey("svc"), GetServiceStatus(sdp.Svc), nil, hpaObject, parent)
	for i := range sdp.Deploy {
		depObject := nodes.AddNodes(sdp.Deploy[i].Name, namespace, handler.GetCrdKey("deploy"), GetDepStatus(sdp.Deploy[i]), svcObject, svcObject)
		if pods, ok := sdp.Pod[sdp.Deploy[i].Name]; ok && pods != nil {
			for j := range pods.Items {
				_ = nodes.AddNodes(pods.Items[j].Name, namespace, handler.GetCrdKey("pod"), GetPodStatus(&pods.Items[i]), svcObject, depObject)
			}
		}
	}
	return svcObject
}
