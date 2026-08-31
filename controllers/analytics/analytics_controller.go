// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analytics

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/analytics"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

const (
	ruleName  = "ossm-operator-usage-rules"
	namespace = "openshift-operators"
)

// Reconciler reconciles operator analytics metrics.
type Reconciler struct {
	client.Client
	Config config.ReconcilerConfig
	Scheme *runtime.Scheme
}

func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Config: cfg,
		Client: client,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=sailoperator.io,resources=istios,verbs=get;list;watch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisions,verbs=get;list;watch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags,verbs=get;list;watch
// +kubebuilder:rbac:groups=sailoperator.io,resources=ztunnels,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;create;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;create;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check if prometheus rule already exists, if not create a new one
	foundRule := &monitoringv1.PrometheusRule{}
	err := r.Get(ctx, types.NamespacedName{Name: ruleName, Namespace: namespace}, foundRule)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new prometheus rule
		prometheusRule := analytics.NewPrometheusRule(namespace)
		if err := r.Create(ctx, prometheusRule); err != nil {
			log.Error(err, "Failed to create prometheus rule")
			return ctrl.Result{}, nil
		}
	}

	if err == nil {
		// Check if prometheus rule spec was changed, if so set as desired
		desiredRuleSpec := analytics.NewPrometheusRuleSpec()
		if !reflect.DeepEqual(foundRule.Spec.DeepCopy(), desiredRuleSpec) {
			desiredRuleSpec.DeepCopyInto(&foundRule.Spec)
			if r.Update(ctx, foundRule); err != nil {
				log.Error(err, "Failed to update prometheus rule")
				return ctrl.Result{}, nil
			}
		}
	}

	// Fetch the Istio instance

	// Fetch the ZTunnel instance

	result, reconcileErr := r.doReconcile(ctx, req)

	return result, reconcileErr
}

func (r *Reconciler) doReconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	istioList := &v1.IstioList{}
	if err := r.List(ctx, istioList); err != nil {
		return ctrl.Result{}, err
	}

	prometheusRule := analytics.NewPrometheusRule(namespace)
	if err := r.Client.Create(ctx, prometheusRule); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("analytics")

	// ownedResourceHandler handles resources that are owned by the IstioRevision CR
	ownedResourceHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.Istio{}, handler.OnlyControllerOwner()))

	namespaceHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Istio{}).
		Named("istiod").
		Watches(&v1.IstioRevision{}, ownedResourceHandler).
		Watches(&corev1.Namespace{}, namespaceHandler, builder.WithPredicates(sidecarInjectionNamespacePredicate())).
		Owns(&monitoringv1.PrometheusRule{}).
		Complete(r)
}

func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	_, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}

	return nil
}

// sidecarInjectionNamespacePredicate returns a predicate that filters namespace events
// to those where istio-injection or istio.io/rev labels are added, removed, or changed.
func sidecarInjectionNamespacePredicate() predicate.Funcs {
	injectionLabelState := func(obj client.Object) string {
		if obj == nil {
			return ""
		}
		labels := obj.GetLabels()
		if labels == nil {
			return ""
		}
		injection := labels[constants.IstioInjectionLabel]
		rev := labels[constants.IstioRevLabel]
		if injection == "" && rev == "" {
			return ""
		}
		return injection + "|" + rev
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return injectionLabelState(e.Object) != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return injectionLabelState(e.ObjectOld) != injectionLabelState(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return injectionLabelState(e.Object) != ""
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return injectionLabelState(e.Object) != ""
		},
	}
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioKind, logger, handler)
}
