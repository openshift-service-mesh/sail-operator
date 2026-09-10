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
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

const namespace = "openshift-operators"

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
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;create;update;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check if a ServiceMonitor already exists, if not create a new one
	foundMonitor := &monitoringv1.ServiceMonitor{}
	if err := r.Get(ctx, types.NamespacedName{Name: analytics.OperatorMonitorName, Namespace: namespace}, foundMonitor); err != nil {
		if apierrors.IsNotFound(err) {
			serviceMonitor := analytics.NewOperatorServiceMonitor(namespace)
			if err := r.Create(ctx, serviceMonitor); err != nil {
				log.Error(err, "Failed to create operator ServiceMonitor")
				return ctrl.Result{}, nil
			}
		}
		log.Error(err, "Failed to get ServiceMonitor")
		return ctrl.Result{}, nil
	}

	// Check if ServiceMonitor spec was changed, if so set as desired
	desiredMonitorSpec := analytics.NewOperatorServiceMonitorSpec()
	if !reflect.DeepEqual(foundMonitor.Spec.DeepCopy(), desiredMonitorSpec) {
		desiredMonitorSpec.DeepCopyInto(&foundMonitor.Spec)
		if err := r.Update(ctx, foundMonitor); err != nil {
			log.Error(err, "Failed to update operator ServiceMonitor")
			return ctrl.Result{}, nil
		}
	}

	// Check if a PrometheusRule already exists, if not create a new one
	foundRule := &monitoringv1.PrometheusRule{}
	if err := r.Get(ctx, types.NamespacedName{Name: analytics.RuleName, Namespace: namespace}, foundRule); err != nil {
		if apierrors.IsNotFound(err) {
			prometheusRule := analytics.NewPrometheusRule(namespace)
			if err := r.Create(ctx, prometheusRule); err != nil {
				log.Error(err, "Failed to create PrometheusRule")
				return ctrl.Result{}, nil
			}
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, nil
	}

	// Check if PrometheusRule spec was changed, if so set as desired
	desiredRuleSpec := analytics.NewPrometheusRuleSpec()
	if !reflect.DeepEqual(foundRule.Spec.DeepCopy(), desiredRuleSpec) {
		desiredRuleSpec.DeepCopyInto(&foundRule.Spec)
		if err := r.Update(ctx, foundRule); err != nil {
			log.Error(err, "Failed to update PrometheusRule")
			return ctrl.Result{}, nil
		}
	}

	// Fetch the Istio instance
	istioList := &v1.IstioList{}
	if err := r.List(ctx, istioList); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to list Istio resource")
		return ctrl.Result{}, nil
	}

	for _, istio := range istioList.Items {
		analytics.IstioVersionTotal.WithLabelValues(istio.Spec.Version).Inc()

		// Check if a ServiceMonitor already exists in istiod control plane namespace, if not create a new one
		foundMonitor := &monitoringv1.ServiceMonitor{}
		if err := r.Get(ctx, types.NamespacedName{Name: analytics.IstiodMonitorName, Namespace: istio.Namespace}, foundMonitor); err != nil {
			if apierrors.IsNotFound(err) {
				serviceMonitor := analytics.NewIstiodServiceMonitor(namespace)
				if err := r.Create(ctx, serviceMonitor); err != nil {
					log.Error(err, "Failed to create istiod ServiceMonitor")
					return ctrl.Result{}, nil
				}
			}
		}
	}

	// Fetch the ZTunnel instance
	ztunnelList := &v1.ZTunnelList{}
	if err := r.List(ctx, ztunnelList); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to list ZTunnel resource")
		return ctrl.Result{}, nil
	}

	for _, ztunnel := range ztunnelList.Items {
		analytics.ZTunnelVersionTotal.WithLabelValues(ztunnel.Spec.Version).Inc()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("analytics")

	// mainObjectHandler handles the IstioRevision watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// ownedResourceHandler handles resources that are owned by the IstioRevision CR
	ownedResourceHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.Istio{}, handler.OnlyControllerOwner()))

	// nsHandler triggers reconciliation in two cases:
	// - when a namespace that is configured with the label `istio-injection=enabled`
	// - when a namespace that is configured with a label `istio.io/rev`
	nsHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	ztunnelHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapZTunnelToReconcileRequests))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("analytics", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).
		Watches(&v1.Istio{}, mainObjectHandler).
		Watches(&v1.IstioRevision{}, ownedResourceHandler).
		Watches(&v1.ZTunnel{}, ztunnelHandler).
		Watches(&corev1.Namespace{},
			nsHandler, builder.WithPredicates(sidecarInjectionNamespacePredicate())).
		Owns(&monitoringv1.ServiceMonitor{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Complete(r)
}

// mapNamespaceToReconcileRequest takes a Namespace event and returns reconcile requests when matching predicate
func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	_, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil
	}

	return []reconcile.Request{{}}
}

// sidecarInjectionNamespacePredicate returns a predicate that filters namespace events
// to those where istio-injection or istio.io/rev labels are added or changed.
func sidecarInjectionNamespacePredicate() predicate.Funcs {
	injectionLabelState := func(obj client.Object) bool {
		if obj == nil {
			return false
		}
		labels := obj.GetLabels()
		if labels == nil {
			return false
		}
		if labels[constants.IstioInjectionLabel] == "" && labels[constants.IstioRevLabel] == "" {
			return false
		}
		if labels[constants.IstioInjectionLabel] == "disabled" {
			return false
		}
		return true
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return injectionLabelState(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return injectionLabelState(e.ObjectOld) != injectionLabelState(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return injectionLabelState(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return injectionLabelState(e.Object)
		},
	}
}

// ambientNamespacePredicate returns a predicate that filters namespace events
// to those where istio.io/dataplane-mode, istio.io/use-waypoint or istio.io/ingress-use-waypoint labels
// are added or changed.
func ambientNamespacePredicate() predicate.Funcs {
	return predicate.Funcs{}
}

// mapZTunnelToReconcileRequests returns reconcile requests for all IstioRevisions that depend on ZTunnel
func (r *Reconciler) mapZTunnelToReconcileRequests(ctx context.Context, _ client.Object) []reconcile.Request {
	list := v1.IstioRevisionList{}
	if err := r.Client.List(ctx, &list); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, rev := range list.Items {
		if revision.DependsOnZTunnel(&rev, r.Config) {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.Name}})
		}
	}
	return reqs
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary("analytics", logger, handler)
}
