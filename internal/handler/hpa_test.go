package kserve

import (
	"context"
	stdlog "log"
	"testing"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kserve-nexus/pkg/client"
	"github.com/kserve-nexus/pkg/log"
	ksvcconstants "github.com/kserve/kserve/pkg/constants"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetAutoscaler(t *testing.T) {
	logger, err := log.NewDefaultZapLogger()
	if err != nil {
		stdlog.Fatal("failed to create root logger", err)
	}
	namespace := "default"
	name := "http-app-scaledobject"
	mgr, cs, err := client.InitK8sClient()
	if err != nil {
		stdlog.Fatal("failed to init k8s client", err)
	}
	kc := mgr.GetClient()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err = mgr.Start(ctx); err != nil {
			stdlog.Fatal("failed to start manager", err)
		}
	}()
	kh := NewKserveHandler(mgr.GetClient(), cs)
	path := "../../../samples/test/hpa/keda.error.yaml"
	if err = applyK8sYaml(path, "default", kc); err != nil {
		t.Fatalf("failed: %+v", err)
	}
	t.Cleanup(func() {
		defer cancel()
		defer func(path, namespace string, c client2.Client) {
			if err = deleteK8sYaml(path, namespace, c); err != nil {
				logger.Error(err, "failed to delete k8s")
			}
		}(path, "default", kc)
	})
	var keda = &kedav1alpha1.ScaledObject{}
	if err = kc.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, keda); err != nil {
		t.Fatalf("failed: %+v", err)
	}

	for i := 0; i <= 10; i++ {
		time.Sleep(time.Duration(i) * time.Second)
		isDone := 0
		for _, c := range keda.Status.Conditions {
			logger.Info("", "type", c.Type, "status", c.Status)
			if c.Type == kedav1alpha1.ConditionActive && c.Status != metav1.ConditionUnknown {
				isDone++
			}
			if c.Type == kedav1alpha1.ConditionReady && c.Status != metav1.ConditionUnknown {
				isDone++
			}
		}
		if isDone == 2 {
			break
		}
	}
	hpa, hpaStatus, keda, kedaStatus := kh.getAutoscaler(ctx, ksvcconstants.AutoscalerClassKeda, "test", name, namespace)

	assert.NotNil(t, hpa)
	assert.Equal(t, metav1.ConditionFalse, hpaStatus)

	assert.NotNil(t, keda)
	assert.Equal(t, metav1.ConditionFalse, kedaStatus)
}
