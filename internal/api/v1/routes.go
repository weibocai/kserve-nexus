package v1

import (
	"github.com/gin-gonic/gin"
	sc "github.com/kserve-nexus/internal/handler"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupRoutes 设置路由
func SetupRoutes(r *gin.Engine, kc client.Client, cs *kubernetes.Clientset) {
	// 定义kserve路由组
	kr := r.Group("/kserve")
	{
		// 创建 UserService 实例
		ks := sc.NewKserveHandler(kc, cs)
		kr.GET("/namespace", ks.ListNamespaces)
		kr.GET("/config", ks.GetConfigMap)
		kr.GET("/isvc", ks.ListIsvc)
		kr.GET("/isvc/:name", ks.GetIsvc)
		kr.GET("/graph", ks.GetGraph)
		kr.GET("/graph/:name", ks.GetGraph)
		kr.GET("/llmisvc", ks.ListLLMIsvc)
		kr.GET("/llmisvc/:name", ks.GetLLMIsvc)
		kr.GET("/crd", ks.GetCrd)
	}
}
