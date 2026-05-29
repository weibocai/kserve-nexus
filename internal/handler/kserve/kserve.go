package kserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kserve-nexus/internal/middleware"
)

// Handler kserve 处理器
type Handler struct {
	kc client.Client
	cs *kubernetes.Clientset
}

// NewKserveHandler 初始化 kserve handler
func NewKserveHandler(kc client.Client, clientSet *kubernetes.Clientset) *Handler {
	return &Handler{kc: kc, cs: clientSet}
}

// GetConfigMap 获取kserve默认配置
// @Summary 获取KServe默认配置
// @Description 获取KServe命名空间下的inferenceservice-config ConfigMap
// @Tags config
// @Accept JSON
// @Produce JSON
// @Success 200 {object} object "配置信息"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/config [get]
func (kh *Handler) GetConfigMap(c *gin.Context) {
	cm := &corev1.ConfigMap{}
	if err := kh.kc.Get(c.Request.Context(), types.NamespacedName{Namespace: ksvcconstants.KServeNamespace, Name: ksvcconstants.InferenceServiceConfigMapName}, cm); err != nil {
		middleware.ErrorJson(c, err, fmt.Sprintf("获取 %s ConfigMap 异常", ksvcconstants.InferenceServiceConfigMapName))
		return
	}
	c.JSON(http.StatusOK, cm)
}

func (kh *Handler) listNamespaces(ctx context.Context) (sort.StringSlice, error) {
	var nsList corev1.NamespaceList
	if err := kh.kc.List(ctx, &nsList); err != nil {
		return nil, err
	}
	namespaces := make(sort.StringSlice, 0, len(nsList.Items))
	for index := range nsList.Items {
		namespaces = append(namespaces, nsList.Items[index].Name)
	}
	sort.Sort(namespaces)
	return namespaces, nil
}

// ListNamespaces 获取所有命名空间
// @Summary 获取所有命名空间
// @Description 获取Kubernetes集群中所有命名空间列表（按字母排序）
// @Tags namespace
// @Accept JSON
// @Produce JSON
// @Success 200 {array} string "命名空间列表"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/namespace [get]
func (kh *Handler) ListNamespaces(c *gin.Context) {
	namespaces, err := kh.listNamespaces(c.Request.Context())
	if err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	c.JSON(http.StatusOK, namespaces)
}

func (kh *Handler) getIsvc(ctx context.Context, namespace string) ([]map[string]string, error) {
	var isvc ksvcv1beta1.InferenceServiceList
	if err := kh.kc.List(ctx, &isvc, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}
	response := make([]map[string]string, len(isvc.Items))
	for i := range isvc.Items {
		svc := isvc.Items[i]
		dm, err := kh.getDeploymentMode(ctx, svc.Status.DeploymentMode, svc.Annotations, nil)
		if err != nil {
			dm = ksvcconstants.Standard
		}
		_response := map[string]string{
			"namespace": svc.Namespace, "name": svc.Name, "deploymentMode": string(dm), "url": svc.Status.URL.String(), "ready": "True", "prev": "",
			"latest": "", "prevRolledoutRevision": "", "latestReadyRevision": "", "age": svc.CreationTimestamp.String(),
		}
		for j := range svc.Status.Conditions {
			if svc.Status.Conditions[j].Type != apis.ConditionReady {
				_response["ready"] = string(svc.Status.Conditions[j].Status)
				_response["reason"] = svc.Status.Conditions[j].Reason
				break
			}
		}
		if cp, ok := svc.Status.Components[ksvcv1beta1.PredictorComponent]; ok {
			for t := range cp.Traffic {
				if (cp.Traffic[t]).Tag == "prev" {
					_response["prev"] = strconv.FormatInt(*(cp.Traffic[t]).Percent, 10)
					_response["prevRolledoutRevision"] = (cp.Traffic[t]).RevisionName
				}
				if *(cp.Traffic[t]).LatestRevision {
					_response["latest"] = strconv.FormatInt(*(cp.Traffic[t]).Percent, 10)
					_response["latestReadyRevision"] = (cp.Traffic[t]).RevisionName
				}
			}
		}
		response[i] = _response
	}
	return response, nil
}

// ListIsvc 获取InferenceService列表
// @Summary 获取InferenceService列表
// @Description 获取指定命名空间下的InferenceService列表，若namespace=all则返回所有命名空间下的服务
// @Tags isvc
// @Accept JSON
// @Produce JSON
// @Param namespace query string false "命名空间，默认all表示所有命名空间"
// @Success 200 {object} middleware.Response "成功"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/isvc [get]
func (kh *Handler) ListIsvc(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "all")
	if namespace != "all" {
		services, err := kh.getIsvc(c.Request.Context(), namespace)
		middleware.ResponseJson(c, services, err)
		return
	}
	namespaces, err := kh.listNamespaces(c.Request.Context())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		//nolint:gocritic
		_ = c.Error(err)
		return
	}
	services := make([]map[string]string, 0)
	for index := range namespaces {
		_services, err := kh.getIsvc(c.Request.Context(), namespaces[index])
		if err != nil {
			middleware.ErrorJson(c, err, "")
			return
		}
		services = append(services, _services...)
	}
	middleware.SuccessJson(c, services)
}

// GetPodLogs 获取pod的日志
func (kh *Handler) GetPodLogs(c *gin.Context) {
	pod := c.Query("pod")
	namespace := c.Query("namespace")
	container := c.DefaultQuery("container", "")
	if pod == "" || namespace == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	tailLines := int64(100) // 只获取最后 100 行
	var plo = &corev1.PodLogOptions{TailLines: &tailLines, Follow: false, Timestamps: true}
	if container != "" {
		plo.Container = container
	}
	req := kh.cs.CoreV1().Pods(namespace).GetLogs(pod, plo)
	podLogs, err := req.Stream(c.Request.Context())
	if err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	defer func() {
		//nolint:gocritic
		_ = podLogs.Close()
	}()
	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, podLogs); err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"logs": buf.String()})
}

// getServiceStandard 标准服务
func (kh *Handler) getServiceStandard(c *gin.Context, isvc *ksvcv1beta1.InferenceService, nodes GraphNodeMap, namespace string) {
	ac := ksvcconstants.AutoscalerClassHPA
	if ac1, ok := isvc.Annotations[ksvcconstants.AutoscalerClass]; ok {
		ac = ksvcconstants.AutoscalerClassType(ac1)
	}
	kh.getHTTPRouteShow(c.Request.Context(), isvc.Name, isvc.Name, namespace, ac, nodes)
	// 推理
	predictorName := ksvcconstants.PredictorServiceName(isvc.Name)
	// 获取存储相关的节点
	storageURIs := make([]string, 0)
	if isvc.Spec.Predictor.Model.StorageURI != nil {
		storageURIs = append(storageURIs, *isvc.Spec.Predictor.Model.StorageURI)
	}
	for _, si := range isvc.Spec.Predictor.StorageUris {
		storageURIs = append(storageURIs, si.Uri)
	}
	_, svcObject := kh.getHTTPRouteShow(c.Request.Context(), isvc.Name, predictorName, namespace, ac, nodes)
	kh.getStorage(c.Request.Context(), isvc.Spec.Predictor.ServiceAccountName, namespace, storageURIs, svcObject, nodes)

	// Transformer
	if isvc.Spec.Transformer != nil {
		transformerName := ksvcconstants.TransformerServiceName(isvc.Name)
		storageURIs = make([]string, 0)
		for _, si := range isvc.Spec.Transformer.StorageUris {
			storageURIs = append(storageURIs, si.Uri)
		}
		_, svcTObject := kh.getHTTPRouteShow(c.Request.Context(), isvc.Name, transformerName, namespace, ac, nodes)
		kh.getStorage(c.Request.Context(), isvc.Spec.Explainer.ServiceAccountName, namespace, storageURIs, svcTObject, nodes)
	}
	// Explainer
	if isvc.Spec.Explainer != nil {
		explainerName := ksvcconstants.ExplainerServiceName(isvc.Name)
		storageURIs = make([]string, 0)
		for _, si := range isvc.Spec.Explainer.StorageUris {
			storageURIs = append(storageURIs, si.Uri)
		}
		_, svcEObject := kh.getHTTPRouteShow(c.Request.Context(), isvc.Name, explainerName, namespace, ac, nodes)
		kh.getStorage(c.Request.Context(), isvc.Spec.Explainer.ServiceAccountName, namespace, storageURIs, svcEObject, nodes)
	}
}

// GetIsvc 获取单个InferenceService详情
// @Summary 获取单个InferenceService详情及拓扑图
// @Description 获取指定命名空间下单个InferenceService的详细信息，包含部署拓扑图。kind=grafana时返回DOT格式，否则返回节点与边JSON
// @Tags isvc
// @Accept JSON
// @Produce JSON
// @Param name path string true "InferenceService名称"
// @Param namespace query string true "命名空间"
// @Param kind query string false "返回格式：grafana(DOT格式)或默认(节点边JSON)"
// @Success 200 {object} middleware.Response "成功"
// @Failure 400 {object} middleware.Response "参数错误"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/isvc/{name} [get]
func (kh *Handler) GetIsvc(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		middleware.ErrorJson(c, nil, fmt.Sprintf("参数错误：name=%s; namespace=%s", name, namespace))
		return
	}

	response := make(map[string]any)
	var isvc ksvcv1beta1.InferenceService
	if err := kh.kc.Get(c.Request.Context(), types.NamespacedName{Namespace: namespace, Name: name}, &isvc); err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	response["isvc"] = isvc
	response["status"] = GraphNodeStatusFalse
	for index := range isvc.Status.Conditions {
		if isvc.Status.Conditions[index].Type == apis.ConditionReady {
			response["status"] = GraphNodeStatusTrue
			break
		}
	}
	cm := &corev1.ConfigMap{}
	if err := kh.kc.Get(c.Request.Context(), types.NamespacedName{Namespace: ksvcconstants.KServeNamespace, Name: ksvcconstants.InferenceServiceConfigMapName}, cm); err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}

	dm, err := kh.getDeploymentMode(c.Request.Context(), cm.Data["deploymentMode"], cm.Data, nil)
	if err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	response["deploymentMode"] = dm
	var nodes GraphNodeMap = make(map[string]*GraphNode)
	if dm == "Standard" {
		kh.getServiceStandard(c, &isvc, nodes, namespace)
	}
	if dm == "Knative" {
		kh.getServiceKnative(c, &isvc, nodes, namespace)
	}
	response["nodes"], response["edges"] = GraphNode2graphNode(nodes)
	middleware.SuccessJson(c, response)
}
