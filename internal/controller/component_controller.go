package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paapv1 "paap/api/v1"
)

const compFinalizer = "paap.io/component-finalizer"

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=paap.io,resources=components,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paap.io,resources=components/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paap.io,resources=components/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps,verbs=get;list;watch;create;update;delete

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	comp := &paapv1.Component{}
	if err := r.Get(ctx, req.NamespacedName, comp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 处理删除
	if !comp.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, comp)
	}

	// 添加 Finalizer
	if !controllerutil.ContainsFinalizer(comp, compFinalizer) {
		controllerutil.AddFinalizer(comp, compFinalizer)
		if err := r.Update(ctx, comp); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 根据 managedBy 决定管理模式
	switch comp.Spec.ManagedBy {
	case "argocd":
		// ArgoCD 管理模式：Deployment 和 Service 都由 ArgoCD 根据 GitOps 仓库管理。
		// Operator 只读状态，避免修改 labels/annotations 导致 Application 永久 OutOfSync。
		logger.Info("component managed by ArgoCD", "name", comp.Name)
	case "operator", "":
		// Operator 管理模式：创建 Deployment + Service
		if err := r.ensureDeployment(ctx, comp); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		if err := r.ensureService(ctx, comp); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
	}

	// 更新 status
	deploy := &appsv1.Deployment{}
	deployKey := types.NamespacedName{
		Name:      componentDeploymentName(comp),
		Namespace: comp.Spec.Deployment.Namespace,
	}
	if err := r.Get(ctx, deployKey, deploy); err == nil {
		comp.Status.Deployment = &paapv1.DeploymentStatus{
			Name:            deploy.Name,
			Namespace:       deploy.Namespace,
			ReadyReplicas:   deploy.Status.ReadyReplicas,
			Replicas:        deploy.Status.Replicas,
			UpdatedReplicas: deploy.Status.UpdatedReplicas,
		}
		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			comp.Status.Phase = "Running"
		} else {
			comp.Status.Phase = "Creating"
		}
	}

	svc := &corev1.Service{}
	svcKey := types.NamespacedName{
		Name:      comp.Spec.Identifier,
		Namespace: comp.Spec.Deployment.Namespace,
	}
	if err := r.Get(ctx, svcKey, svc); err == nil {
		comp.Status.Service = &paapv1.ServiceStatus{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			ClusterIP: svc.Spec.ClusterIP,
		}
	}

	comp.Status.ObservedGeneration = comp.Generation
	if err := r.Status().Update(ctx, comp); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ComponentReconciler) handleDeletion(ctx context.Context, comp *paapv1.Component) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(comp, compFinalizer) {
		return ctrl.Result{}, nil
	}

	for _, appKey := range componentArgoCDApplicationDeleteKeys(comp) {
		app := &unstructured.Unstructured{}
		app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
		if err := r.Get(ctx, appKey, app); err == nil {
			logger.Info("deleting ArgoCD Application", "namespace", appKey.Namespace, "name", appKey.Name)
			if err := r.Delete(ctx, app); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		} else if err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// 删除 Deployment。ArgoCD 管理的组件实际 Deployment 名是 identifier，
	// 旧 operator 管理模式则是 identifier-namespace，删除时两个都兜底清理。
	for _, deployName := range componentDeploymentDeleteNames(comp) {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: deployName, Namespace: comp.Spec.Deployment.Namespace}, deploy); err == nil {
			logger.Info("deleting Deployment", "name", deployName)
			if err := r.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
	}

	// 删除 Service
	svcName := comp.Spec.Identifier
	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: comp.Spec.Deployment.Namespace}, svc); err == nil {
		logger.Info("deleting Service", "name", svcName)
		r.Delete(ctx, svc)
	}

	for _, selector := range []map[string]string{
		{"paap.io/component": comp.Spec.Identifier},
		{"app": comp.Spec.Identifier},
	} {
		if err := r.deleteRuntimeResourcesByLabels(ctx, comp.Spec.Deployment.Namespace, selector); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 移除 Finalizer
	controllerutil.RemoveFinalizer(comp, compFinalizer)
	if err := r.Update(ctx, comp); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Component deleted", "name", comp.Name)
	return ctrl.Result{}, nil
}

func (r *ComponentReconciler) deleteRuntimeResourcesByLabels(ctx context.Context, namespace string, selector map[string]string) error {
	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range deployments.Items {
		if err := r.Delete(ctx, &deployments.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	replicaSets := &appsv1.ReplicaSetList{}
	if err := r.List(ctx, replicaSets, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range replicaSets.Items {
		if err := r.Delete(ctx, &replicaSets.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range pods.Items {
		if err := r.Delete(ctx, &pods.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	services := &corev1.ServiceList{}
	if err := r.List(ctx, services, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}
	for i := range services.Items {
		if err := r.Delete(ctx, &services.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func componentDeploymentName(comp *paapv1.Component) string {
	if comp.Spec.ManagedBy == "argocd" {
		return comp.Spec.Identifier
	}
	return comp.Spec.Identifier + "-" + comp.Spec.Deployment.Namespace
}

func componentDeploymentDeleteNames(comp *paapv1.Component) []string {
	primary := componentDeploymentName(comp)
	legacy := comp.Spec.Identifier + "-" + comp.Spec.Deployment.Namespace
	if primary == legacy {
		return []string{primary}
	}
	return []string{primary, legacy}
}

func componentArgoCDApplicationDeleteKeys(comp *paapv1.Component) []types.NamespacedName {
	if comp.Spec.ManagedBy != "argocd" {
		return nil
	}
	appIdentifier := strings.TrimSpace(comp.Labels["paap.io/app"])
	if appIdentifier == "" {
		appIdentifier = strings.TrimPrefix(comp.Namespace, "paap-app-")
	}
	envIdentifier := strings.TrimSpace(comp.Labels["paap.io/env"])
	if envIdentifier == "" {
		envIdentifier = strings.TrimSpace(comp.Spec.EnvironmentRef.Name)
	}

	names := []string{}
	if comp.Spec.ArgoCDAppRef != nil && strings.TrimSpace(comp.Spec.ArgoCDAppRef.Name) != "" {
		names = append(names, strings.TrimSpace(comp.Spec.ArgoCDAppRef.Name))
	}
	if appIdentifier != "" && envIdentifier != "" && strings.TrimSpace(comp.Spec.Identifier) != "" {
		names = append(names, fmt.Sprintf("%s-%s-%s", appIdentifier, envIdentifier, comp.Spec.Identifier))
	}

	namespaces := []string{}
	if appIdentifier != "" && envIdentifier != "" {
		namespaces = append(namespaces,
			fmt.Sprintf("%s-%s-argocd", appIdentifier, envIdentifier),
			fmt.Sprintf("%s-%s-deploy", appIdentifier, envIdentifier),
		)
	}
	keys := make([]types.NamespacedName, 0, len(names)*len(namespaces))
	seen := map[string]bool{}
	for _, namespace := range namespaces {
		for _, name := range names {
			key := types.NamespacedName{Namespace: namespace, Name: name}
			seenKey := key.String()
			if key.Namespace == "" || key.Name == "" || seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func (r *ComponentReconciler) ensureDeployment(ctx context.Context, comp *paapv1.Component) error {
	deployName := comp.Spec.Identifier + "-" + comp.Spec.Deployment.Namespace
	deploy := &appsv1.Deployment{}
	deployKey := types.NamespacedName{Name: deployName, Namespace: comp.Spec.Deployment.Namespace}
	envVars := componentDeploymentEnvVars(comp)
	labels := componentResourceLabels(comp)
	annotations := componentResourceAnnotations(comp)

	if err := r.Get(ctx, deployKey, deploy); err == nil {
		// 更新 Deployment（如果镜像/副本变化）
		changed := false
		if mergeObjectLabels(deploy, labels) {
			changed = true
		}
		if mergeObjectLabels(&deploy.Spec.Template, labels) {
			changed = true
		}
		if mergeObjectAnnotations(deploy, annotations) {
			changed = true
		}
		if mergeObjectAnnotations(&deploy.Spec.Template, annotations) {
			changed = true
		}
		if *deploy.Spec.Replicas != comp.Spec.Deployment.Replicas {
			deploy.Spec.Replicas = &comp.Spec.Deployment.Replicas
			changed = true
		}
		if deploy.Spec.Template.Spec.Containers[0].Image != comp.Spec.Deployment.Image+":"+comp.Spec.Deployment.Tag {
			deploy.Spec.Template.Spec.Containers[0].Image = comp.Spec.Deployment.Image + ":" + comp.Spec.Deployment.Tag
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Env, envVars) {
			deploy.Spec.Template.Spec.Containers[0].Env = envVars
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Command, comp.Spec.Deployment.Command) {
			deploy.Spec.Template.Spec.Containers[0].Command = comp.Spec.Deployment.Command
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Args, comp.Spec.Deployment.Args) {
			deploy.Spec.Template.Spec.Containers[0].Args = comp.Spec.Deployment.Args
			changed = true
		}
		if changed {
			return r.Update(ctx, deploy)
		}
		return nil
	}

	// 创建 Deployment
	deploy = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        deployName,
			Namespace:   comp.Spec.Deployment.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &comp.Spec.Deployment.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": comp.Spec.Identifier},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: mergeMaps(annotations, map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port": func() string {
							if comp.Spec.Service != nil {
								return fmt.Sprintf("%d", comp.Spec.Service.TargetPort)
							}
							return "8080"
						}(),
					}),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    comp.Spec.Identifier,
							Image:   comp.Spec.Deployment.Image + ":" + comp.Spec.Deployment.Tag,
							Ports:   []corev1.ContainerPort{},
							Env:     envVars,
							Command: comp.Spec.Deployment.Command,
							Args:    comp.Spec.Deployment.Args,
						},
					},
				},
			},
		},
	}

	if comp.Spec.Service != nil {
		deploy.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: comp.Spec.Service.TargetPort},
		}
	}

	return r.Create(ctx, deploy)
}

func componentResourceLabels(comp *paapv1.Component) map[string]string {
	appIdentifier := strings.TrimSpace(comp.Labels["paap.io/app"])
	envIdentifier := strings.TrimSpace(comp.Labels["paap.io/env"])
	componentID := strings.TrimSpace(comp.Spec.Identifier)
	componentType := strings.TrimSpace(comp.Spec.Type)
	return map[string]string{
		"app":                    componentID,
		"paap.io/app":            appIdentifier,
		"paap.io/env":            envIdentifier,
		"paap.io/component":      componentID,
		"paap.io/component-type": componentType,
		"paap.io/category":       "component",
		"paap.io/resource-role":  "component",
		"paap.io/managed-by":     "paap-operator",
	}
}

func componentResourceAnnotations(comp *paapv1.Component) map[string]string {
	labels := componentResourceLabels(comp)
	delete(labels, "app")
	labels["paap.io/component-name"] = strings.TrimSpace(comp.Spec.Name)
	return labels
}

func mergeMaps(base map[string]string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func componentDeploymentEnvVars(comp *paapv1.Component) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0, len(comp.Spec.Deployment.Env))
	for _, env := range comp.Spec.Deployment.Env {
		ev := corev1.EnvVar{Name: env.Name, Value: env.Value}
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			ev.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: env.ValueFrom.SecretKeyRef.Name},
					Key:                  env.ValueFrom.SecretKeyRef.Key,
				},
			}
		}
		if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
			ev.ValueFrom = &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: env.ValueFrom.ConfigMapKeyRef.Name},
					Key:                  env.ValueFrom.ConfigMapKeyRef.Key,
				},
			}
		}
		envVars = append(envVars, ev)
	}
	return envVars
}

func (r *ComponentReconciler) ensureService(ctx context.Context, comp *paapv1.Component) error {
	if comp.Spec.Service == nil {
		return nil
	}

	svcName := comp.Spec.Identifier
	svc := &corev1.Service{}
	svcKey := types.NamespacedName{Name: svcName, Namespace: comp.Spec.Deployment.Namespace}
	labels := componentResourceLabels(comp)
	annotations := componentResourceAnnotations(comp)

	if err := r.Get(ctx, svcKey, svc); err == nil {
		changed := mergeObjectLabels(svc, labels)
		changed = mergeObjectAnnotations(svc, annotations) || changed
		if changed {
			return r.Update(ctx, svc)
		}
		return nil
	}

	svcType := corev1.ServiceTypeClusterIP
	if comp.Spec.Service.Type != "" {
		svcType = corev1.ServiceType(comp.Spec.Service.Type)
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   comp.Spec.Deployment.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: svcType,
			Selector: map[string]string{
				"app": comp.Spec.Identifier,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       comp.Spec.Service.Port,
					TargetPort: intstr.FromInt(int(comp.Spec.Service.TargetPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	return r.Create(ctx, svc)
}

func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paapv1.Component{}).
		Complete(r)
}
