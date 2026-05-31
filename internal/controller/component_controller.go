package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		// ArgoCD 管理模式：只创建 Service，Deployment 交给 ArgoCD
		logger.Info("component managed by ArgoCD", "name", comp.Name)
		if err := r.ensureService(ctx, comp); err != nil {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
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
		Name:      comp.Spec.Identifier + "-" + comp.Spec.Deployment.Namespace,
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

	// 删除 Deployment
	deployName := comp.Spec.Identifier + "-" + comp.Spec.Deployment.Namespace
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployName, Namespace: comp.Spec.Deployment.Namespace}, deploy); err == nil {
		logger.Info("deleting Deployment", "name", deployName)
		r.Delete(ctx, deploy)
	}

	// 删除 Service
	svcName := comp.Spec.Identifier
	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: comp.Spec.Deployment.Namespace}, svc); err == nil {
		logger.Info("deleting Service", "name", svcName)
		r.Delete(ctx, svc)
	}

	// 移除 Finalizer
	controllerutil.RemoveFinalizer(comp, compFinalizer)
	if err := r.Update(ctx, comp); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Component deleted", "name", comp.Name)
	return ctrl.Result{}, nil
}

func (r *ComponentReconciler) ensureDeployment(ctx context.Context, comp *paapv1.Component) error {
	deployName := comp.Spec.Identifier + "-" + comp.Spec.Deployment.Namespace
	deploy := &appsv1.Deployment{}
	deployKey := types.NamespacedName{Name: deployName, Namespace: comp.Spec.Deployment.Namespace}

	if err := r.Get(ctx, deployKey, deploy); err == nil {
		// 更新 Deployment（如果镜像/副本变化）
		changed := false
		if *deploy.Spec.Replicas != comp.Spec.Deployment.Replicas {
			deploy.Spec.Replicas = &comp.Spec.Deployment.Replicas
			changed = true
		}
		if deploy.Spec.Template.Spec.Containers[0].Image != comp.Spec.Deployment.Image+":"+comp.Spec.Deployment.Tag {
			deploy.Spec.Template.Spec.Containers[0].Image = comp.Spec.Deployment.Image + ":" + comp.Spec.Deployment.Tag
			changed = true
		}
		if changed {
			return r.Update(ctx, deploy)
		}
		return nil
	}

	// 创建 Deployment
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
		envVars = append(envVars, ev)
	}

	deploy = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: comp.Spec.Deployment.Namespace,
			Labels: map[string]string{
				"app":                comp.Spec.Identifier,
				"paap.io/component": comp.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &comp.Spec.Deployment.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": comp.Spec.Identifier},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": comp.Spec.Identifier},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port": func() string {
							if comp.Spec.Service != nil {
								return fmt.Sprintf("%d", comp.Spec.Service.TargetPort)
							}
							return "8080"
						}(),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  comp.Spec.Identifier,
							Image: comp.Spec.Deployment.Image + ":" + comp.Spec.Deployment.Tag,
							Ports: []corev1.ContainerPort{},
							Env:   envVars,
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

func (r *ComponentReconciler) ensureService(ctx context.Context, comp *paapv1.Component) error {
	if comp.Spec.Service == nil {
		return nil
	}

	svcName := comp.Spec.Identifier
	svc := &corev1.Service{}
	svcKey := types.NamespacedName{Name: svcName, Namespace: comp.Spec.Deployment.Namespace}

	if err := r.Get(ctx, svcKey, svc); err == nil {
		return nil
	}

	svcType := corev1.ServiceTypeClusterIP
	if comp.Spec.Service.Type != "" {
		svcType = corev1.ServiceType(comp.Spec.Service.Type)
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: comp.Spec.Deployment.Namespace,
			Labels: map[string]string{
				"app":                comp.Spec.Identifier,
				"paap.io/component": comp.Name,
			},
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
