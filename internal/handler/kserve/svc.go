package kserve

import (
	"context"

	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/pkg/log"
	"github.com/kserve-nexus/pkg/utils"
)

// service、develop、pod 对应关系
type Service struct {
	Svc        *corev1.Service            // service
	Deploy     []*appsv1.Deployment       // deployment
	Pod        map[string]*corev1.PodList // deployment pods
	Autoscaler *Autoscaler

	Name      string
	Namespace string
	IsvcName  string
}

// service、develop、pod 转换成图结构
func (s *Service) ToGraphNode(ctx context.Context, nodes *GraphNodeMap, belong *GraphNode, parent ...*GraphNode) (*GraphNode, *GraphNode) {
	aHead, aTail := s.Autoscaler.ToGraphNode(nodes, belong, parent...)
	svcObject := nodes.AddNodes(s.Name, s.Namespace, utils.GetCrdKey("svc"), GetServiceStatus(s.Svc), belong, aTail)
	for i := range s.Deploy {
		depObject := nodes.AddNodes(s.Deploy[i].Name, s.Namespace, utils.GetCrdKey("deploy"), GetDepStatus(s.Deploy[i]), svcObject, svcObject)
		if pods, ok := s.Pod[s.Deploy[i].Name]; ok && pods != nil {
			for j := range pods.Items {
				_ = nodes.AddNodes(pods.Items[j].Name, s.Namespace, utils.GetCrdKey("pod"), GetPodStatus(&pods.Items[i]), svcObject, depObject)
			}
		}
	}
	return aHead, nil
}

// NewService 获取 service、deployment、pods
func NewService(ctx context.Context, kc client.Client, isvcName, name, namespace string, ac ksvcconstants.AutoscalerClassType) *Service {
	sdp := Service{
		Svc: nil, Deploy: make([]*appsv1.Deployment, 0), Pod: make(map[string]*corev1.PodList), Autoscaler: NewAutoscaler(ctx, kc, ac, isvcName, name, namespace),
	}

	// 获取 service 信息
	svc := &corev1.Service{}
	if err := kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, svc); err != nil {
		log.Logger.Error(err, "获取 service 失败", "name", name, "namespace", namespace)
		return &sdp
	}
	sdp.Svc = svc

	// 根据标签获取deployment
	dep := &appsv1.DeploymentList{}
	if err := kc.List(ctx, dep, client.InNamespace(namespace), client.MatchingLabels(svc.Spec.Selector)); err != nil {
		log.Logger.Error(err, "获取 deployment 失败", "name", name, "namespace", namespace)
		return &sdp
	}

	// 获取每个deployment对应的pods
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
