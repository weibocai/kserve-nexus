package kserve

import (
	"context"
	"strings"

	"github.com/kserve-nexus/pkg/log"
	ksvcv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// getStoragePvc 获取pvc的链，kserve只允许有一个pvc存储
func (kh *KserveHandler) getStoragePvc(ctx context.Context, storageUri, namespace string, nodes simpleObjectMap) {
	pvcName := strings.Split(strings.TrimPrefix(storageUri, "pvc://"), "/")[0]
	pvc := &corev1.PersistentVolumeClaim{}
	// 查找对于的pvc
	pvcObj := nodes.AddNodes(pvcName, namespace, GetCrdKey("pvc"), isvcGraphNodeStatusTrue, nil)
	if err := kh.kc.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: namespace}, pvc); err != nil {
		log.Logger.Error(err, "Failed to get pvc")
		pvcObj.SetStatus(isvcGraphNodeStatusFalse)
		return
	}

	pvName := pvc.Spec.VolumeName
	pv := &corev1.PersistentVolume{}
	// 查找pv
	pvObj := nodes.AddNodes(pvName, "", GetCrdKey("pv"), isvcGraphNodeStatusTrue, nil)
	pvcObj.AddParent(pvObj)
	if err := kh.kc.Get(ctx, types.NamespacedName{Name: pvName, Namespace: namespace}, pv); err != nil {
		pvcObj.SetStatus(isvcGraphNodeStatusFalse)
		log.Logger.Error(err, "Failed to get pv")
		return
	}
	// 查找 StorageClass
	scObj := nodes.AddNodes("未知", "", GetCrdKey("sc"), isvcGraphNodeStatusTrue, nil)
	pvObj.AddParent(scObj)
	if pvc.Spec.StorageClassName != nil {
		sc := &storagev1.StorageClass{}
		scObj.Name = *pvc.Spec.StorageClassName
		if err := kh.kc.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName, Namespace: namespace}, sc); err != nil {
			log.Logger.Error(err, "Failed to get sc")
			scObj.SetStatus(isvcGraphNodeStatusFalse)
		}
		return
	}
	// 如果没有指定，这找到系统默认的StorageClass
	scList := &storagev1.StorageClassList{}
	if err := kh.kc.List(ctx, scList); err != nil {
		log.Logger.Error(err, "Failed to list sc")
	} else {
		for index := range scList.Items {
			if _, ok := scList.Items[index].Annotations["storageclass.kubernetes.io/is-default-class"]; ok {
				scObj.Name = scList.Items[index].Name
				return
			}
		}
	}
	scObj.SetStatus(isvcGraphNodeStatusFalse)
}

// getStorageSecret 获取相关的密钥信息
func (kh *KserveHandler) getStorageSecret(ctx context.Context, serviceAccountName string, parent *simpleObject, nodes simpleObjectMap) {
	if serviceAccountName == "" {
		return
	}
	var sa = &corev1.ServiceAccount{}
	var saObj = nodes.AddNodes(serviceAccountName, "", GetCrdKey("ServiceAccount"), isvcGraphNodeStatusTrue, nil, parent)
	if err := kh.kc.Get(ctx, types.NamespacedName{Name: serviceAccountName}, sa); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		saObj.SetStatus(isvcGraphNodeStatusFalse)
		log.Logger.Error(err, "get service account failed", "serviceAccountName", serviceAccountName)
		return
	}
	for _, secret := range sa.Secrets {
		ss := &corev1.Secret{}
		var ssObj = nodes.AddNodes(secret.Name, secret.Namespace, GetCrdKey("Secret"), isvcGraphNodeStatusTrue, nil, saObj)
		if err := kh.kc.Get(ctx, types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}, ss); err != nil {
			ssObj.SetStatus(isvcGraphNodeStatusFalse)
		}
	}
}

// getStorage 存储相关的节点
func (kh *KserveHandler) getStorage(ctx context.Context, serviceAccountName, namespace string, storageURIs []string, parent *simpleObject, nodes simpleObjectMap) {
	var unPvc = ""
	// 遍历 storageURIs，找出pvc以及其他的uri，分类处理
	for _, storageUri := range storageURIs {
		if strings.HasPrefix(storageUri, "pvc://") {
			kh.getStoragePvc(ctx, storageUri, namespace, nodes)
		} else {
			if unPvc == "" {
				unPvc = storageUri
			}
		}
	}

	// 找到非pvc的路由
	if unPvc == "" {
		return
	}
	storageContainers := &ksvcv1alpha1.ClusterStorageContainerList{}
	if err := kh.kc.List(ctx, storageContainers); err != nil {
		log.Logger.Error(err, "Failed to list storage containers")
		return
	}
	var tsc *ksvcv1alpha1.ClusterStorageContainer
	for i := range storageContainers.Items {
		sc := storageContainers.Items[i]
		if sc.IsDisabled() {
			continue
		}
		if sc.Spec.WorkloadType != ksvcv1alpha1.InitContainer {
			continue
		}
		if supported, err := sc.Spec.IsStorageUriSupported(unPvc); err != nil || !supported {
			if err != nil {
				log.Logger.Error(err, "IsStorageUriSupported", "StorageUri", unPvc)
			}
			continue
		}
		tsc = &sc
		break
	}
	name, status := "未知", isvcGraphNodeStatusFalse
	if tsc != nil {
		name, status = tsc.Name, isvcGraphNodeStatusTrue
	}
	tscObj := nodes.AddNodes(name, "", GetCrdKey(""), status, nil, parent)
	kh.getStorageSecret(ctx, serviceAccountName, tscObj, nodes)
}
