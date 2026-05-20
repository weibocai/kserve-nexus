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

type svcDepPod struct {
	Svc    *corev1.Service
	Deploy []*appsv1.Deployment
	Pod    map[string]*corev1.PodList
}

func getServiceStatus(svc *corev1.Service) isvcGraphNodeStatus {
	if svc == nil {
		return isvcGraphNodeStatusFalse
	}
	return isvcGraphNodeStatusTrue
}

func getPodStatus(pod *corev1.Pod) isvcGraphNodeStatus {
	podReady := isvcGraphNodeStatusFalse
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			podReady = isvcGraphNodeStatusTrue
			break
		}
	}
	return podReady
}

func getDepStatus(dep *appsv1.Deployment) isvcGraphNodeStatus {
	if dep != nil && dep.Status.AvailableReplicas == *dep.Spec.Replicas {
		return isvcGraphNodeStatusTrue
	}
	return isvcGraphNodeStatusFalse
}

// getSvcDep 获取推理服务相关信息：service、deployment、pods
func (kh *Handler) getSvcDep(ctx context.Context, name, namespace string) svcDepPod {
	sdp := svcDepPod{
		nil, make([]*appsv1.Deployment, 0), make(map[string]*corev1.PodList),
	}

	svc := &corev1.Service{}
	if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, svc); err != nil {
		log.Logger.Error(err, "获取 service 失败", "name", name, "namespace", namespace)
		return sdp
	}
	sdp.Svc = svc
	dep := &appsv1.DeploymentList{}
	if err := kh.kc.List(ctx, dep, client.InNamespace(namespace), client.MatchingLabels(svc.Spec.Selector)); err != nil {
		log.Logger.Error(err, "获取 deployment 失败", "name", name, "namespace", namespace)
		return sdp
	}
	for i := range dep.Items {
		sdp.Deploy = append(sdp.Deploy, &dep.Items[i])
		pods := &corev1.PodList{}
		ls, err := metav1.LabelSelectorAsSelector(dep.Items[i].Spec.Selector)
		if err != nil {
			log.Logger.Error(err, "获取 pods 失败", "name", name, "namespace", namespace)
			continue
		}
		if err = kh.kc.List(ctx, pods, &client.ListOptions{Namespace: namespace}, &client.MatchingLabelsSelector{Selector: ls}); err != nil {
			log.Logger.Error(err, "获取 pods 失败", "name", name, "namespace", namespace)
			continue
		}
		sdp.Pod[dep.Items[i].Name] = pods
	}
	return sdp
}

// service、develop、pod关系
func (kh *Handler) getSvcDepShow(ctx context.Context, isvcName, name, namespace string, parent *simpleObject, ac ksvcconstants.AutoscalerClassType, nodes simpleObjectMap) *simpleObject {
	var hpaObject *simpleObject = nil
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

	sdp := kh.getSvcDep(ctx, name, namespace)
	svcObject := nodes.AddNodes(name, namespace, handler.GetCrdKey("svc"), getServiceStatus(sdp.Svc), nil, hpaObject, parent)
	for i := range sdp.Deploy {
		depObject := nodes.AddNodes(sdp.Deploy[i].Name, namespace, handler.GetCrdKey("deploy"), getDepStatus(sdp.Deploy[i]), svcObject, svcObject)
		if pods, ok := sdp.Pod[sdp.Deploy[i].Name]; ok && pods != nil {
			for j := range pods.Items {
				_ = nodes.AddNodes(pods.Items[j].Name, namespace, handler.GetCrdKey("pod"), getPodStatus(&pods.Items[i]), svcObject, depObject)
			}
		}
	}
	return svcObject
}
