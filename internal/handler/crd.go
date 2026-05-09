package kserve

import (
	"github.com/gin-gonic/gin"
	"github.com/kserve-nexus/internal/middleware"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCrd 获取CRD资源
// @Summary 获取指定CRD资源
// @Description 根据kind、namespace和name获取指定的Kubernetes CRD资源（目前支持kind=pod）
// @Tags crd
// @Accept json
// @Produce json
// @Param name query string true "资源名称"
// @Param kind query string true "资源类型（如pod）"
// @Param namespace query string true "命名空间"
// @Success 200 {object} middleware.Response "成功"
// @Failure 400 {object} middleware.Response "参数错误"
// @Failure 500 {object} middleware.Response "请求异常"
// @Router /kserve/crd [get]
func (kh *KserveHandler) GetCrd(c *gin.Context) {
	name := c.Query("name")
	kind := c.Query("kind")
	namespace := c.Query("namespace")
	if kind == "" || namespace == "" || name == "" {
		middleware.ErrorJson(c, nil, "kind or namespace or name is empty")
		return
	}
	var obj client.Object
	switch kind {
	case "pod":
		obj = &v1.Pod{}
	case "pvc":
		obj = &v1.PersistentVolumeClaim{}
	}
	if err := kh.kc.Get(c.Request.Context(), types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		middleware.ErrorJson(c, err, "")
		return
	}
	middleware.SuccessJson(c, map[string]any{"kind": kind, "crd": obj})
}
