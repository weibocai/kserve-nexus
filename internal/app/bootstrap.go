package app

import (
	"fmt"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	ctrl "sigs.k8s.io/controller-runtime"

	_ "github.com/kserve-nexus/docs"
	"github.com/kserve-nexus/internal/api/v1"
	"github.com/kserve-nexus/internal/middleware"
	"github.com/kserve-nexus/pkg/client"
	"github.com/kserve-nexus/pkg/log"
)

// Start 启动服务
func Start() {
	mgr, cs, err := client.InitK8sClient()
	if err != nil {
		log.Logger.Error(err, "初始化 k8s client 失败")
	}
	r := gin.New()
	r.Use(middleware.Recovery())
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	v1.SetupRoutes(r, mgr.GetClient(), cs)
	go func() {
		if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			log.Logger.Error(err, "failed to start server")
		}
		log.Logger.Info("k8s client 连接断开")
	}()
	if err = r.Run(fmt.Sprintf(":%d", 5109)); err != nil {
		log.Logger.Error(err, "服务启动失败")
	}
}
