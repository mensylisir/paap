package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paapv1 "paap/api/v1"
)

const (
	appFinalizer = "paap.io/application-finalizer"
	crNamespace  = "paap-system" // Application CR 所在 namespace
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=paap.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paap.io,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paap.io,resources=applications/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;delete

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	app := &paapv1.Application{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 处理删除
	if !app.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, app)
	}

	// 添加 Finalizer
	if !controllerutil.ContainsFinalizer(app, appFinalizer) {
		controllerutil.AddFinalizer(app, appFinalizer)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 创建 CR namespace (paap-app-{identifier})
	crNSName := fmt.Sprintf("paap-app-%s", app.Spec.Identifier)
	if err := r.ensureCRNamespace(ctx, app, crNSName); err != nil {
		logger.Error(err, "failed to ensure CR namespace", "namespace", crNSName)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// 汇总 Environment 状态
	envs, err := r.listEnvironments(ctx, crNSName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 更新 status
	app.Status.Phase = "Active"
	app.Status.EnvironmentCount = len(envs)
	app.Status.Environments = make([]paapv1.EnvironmentSummary, 0, len(envs))
	for _, env := range envs {
		app.Status.Environments = append(app.Status.Environments, paapv1.EnvironmentSummary{
			Name:      env.Spec.Name,
			Namespace: env.Spec.PrimaryNamespace,
			Phase:     env.Status.Phase,
		})
	}

	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ApplicationReconciler) handleDeletion(ctx context.Context, app *paapv1.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(app, appFinalizer) {
		return ctrl.Result{}, nil
	}

	crNSName := fmt.Sprintf("paap-app-%s", app.Spec.Identifier)

	// 删除所有 Environment CR（会触发 Environment Controller 的级联删除）
	envs, err := r.listEnvironments(ctx, crNSName)
	if err == nil {
		for i := range envs {
			logger.Info("deleting Environment CR", "name", envs[i].Name, "namespace", crNSName)
			if err := r.Delete(ctx, &envs[i]); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, err
			}
		}
	}

	// 检查是否还有 Environment CR
	remaining, _ := r.listEnvironments(ctx, crNSName)
	if len(remaining) > 0 {
		// 等待子 CR 全部删除
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	// 删除 CR namespace
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: crNSName}, ns); err == nil {
		logger.Info("deleting CR namespace", "namespace", crNSName)
		if err := r.Delete(ctx, ns); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, err
		}
	}

	// 移除 Finalizer
	controllerutil.RemoveFinalizer(app, appFinalizer)
	if err := r.Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Application deleted", "name", app.Name)
	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) ensureCRNamespace(ctx context.Context, app *paapv1.Application, nsName string) error {
	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: nsName}, ns)
	if err == nil {
		return nil // 已存在
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// 创建 namespace
	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"paap.io/app":        app.Spec.Identifier,
				"paap.io/managed-by": "paap-operator",
			},
		},
	}
	return r.Create(ctx, ns)
}

func (r *ApplicationReconciler) listEnvironments(ctx context.Context, namespace string) ([]paapv1.Environment, error) {
	envList := &paapv1.EnvironmentList{}
	if err := r.List(ctx, envList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return envList.Items, nil
}

func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paapv1.Application{}).
		Complete(r)
}
