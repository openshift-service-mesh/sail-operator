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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const monitorName = "sail-operator-controller-manager-metrics-monitor"

// NewServiceMonitor creates a ServiceMonitor(CR) for scraping metrics from the operator
func NewServiceMonitor(namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      monitorName,
			Namespace: namespace,
		},
		Spec: *NewServiceMonitorSpec(),
	}
}

// NewServiceMonitorSpec creates ServiceMonitorSpec for scraping metrics
func NewServiceMonitorSpec() *monitoringv1.ServiceMonitorSpec {
	return &monitoringv1.ServiceMonitorSpec{
		Endpoints: []monitoringv1.Endpoint{{
			BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			Path:            "/metrics",
			Port:            "https",
		}},
		Selector: metav1.LabelSelector{MatchLabels: map[string]string{"control-plane": "servicemesh-operator3"}},
	}
}
