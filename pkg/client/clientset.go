package client

import (
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	ksvcv1alpha2 "github.com/kserve/kserve/pkg/apis/serving/v1alpha2"
	ksvcv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knnetworkingv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kserve-nexus/pkg/log"
)

var setupLog = ctrl.Log.WithName("setup")

type addToSchemeFunc func(scheme *runtime.Scheme) error

// InitK8sClient 初始k8s连接器
func InitK8sClient() (manager.Manager, *kubernetes.Clientset, error) {
	ctrl.SetLogger(log.Logger)
	scheme := runtime.NewScheme()
	// 注册需要的scheme
	registerScheme := []addToSchemeFunc{
		corev1.AddToScheme, appsv1.AddToScheme, // 内置
		ksvcv1beta1.AddToScheme, ksvcv1alpha1.AddToScheme, ksvcv1alpha2.AddToScheme,
		kedav1alpha1.AddToScheme, autoscalingv2.AddToScheme, // 自动缩放
		gwapiv1.Install,                  // 网关
		istioclientv1beta1.AddToScheme,   // istio 服务网格
		knnetworkingv1alpha1.AddToScheme, // Serverless
	}
	for _, as := range registerScheme {
		if err := as(scheme); err != nil {
			return nil, nil, err
		}
	}

	stripManagedFields := func(obj interface{}) (interface{}, error) {
		// 注意：这里需要类型断言，通常处理 metav1.Object 接口
		if accessor, ok := obj.(metav1.Object); ok {
			// 将 ManagedFields 设置为 nil，Informer 存入缓存时就不会保留它
			accessor.SetManagedFields(nil)
		}
		return obj, nil
	}
	defaultUnsafeDisableDeepCopy := true
	ctr := ctrl.GetConfigOrDie()

	// 管理器+缓存
	mgr, err := ctrl.NewManager(ctr, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultTransform:             stripManagedFields,            // 全局 Transform：对所有缓存对象应用“剥离大字段”逻辑
			DefaultUnsafeDisableDeepCopy: &defaultUnsafeDisableDeepCopy, // 主要是读操作，禁用 deep copy
			SyncPeriod:                   func() *time.Duration { d := 1 * time.Minute; return &d }(),
		},
	})
	if err != nil {
		return nil, nil, err
	}

	// 无缓存客户端
	clientSet, err := kubernetes.NewForConfig(ctr)
	if err != nil {
		return nil, nil, err
	}
	return mgr, clientSet, nil
}
