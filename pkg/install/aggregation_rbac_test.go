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

package install

import (
	"context"
	"testing"

	"github.com/istio-ecosystem/sail-operator/chart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newFakeRBACClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = rbacv1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func newTestCRDManager() *crdManager {
	return &crdManager{crdFS: chart.CRDsFS}
}

func TestReconcileAggregationClusterRoles_Creates(t *testing.T) {
	cl := newFakeRBACClient()
	revision := "default"

	err := reconcileAggregationClusterRoles(context.Background(), cl, newTestCRDManager(), revision)
	require.NoError(t, err)

	for _, ar := range aggregationRoles {
		name := aggregationClusterRoleName(revision, ar.suffix)
		cr := &rbacv1.ClusterRole{}
		err := cl.Get(context.Background(), client.ObjectKey{Name: name}, cr)
		require.NoError(t, err, "ClusterRole %s should exist", name)

		assert.Equal(t, managedByValue, cr.Labels[managedByLabelKey])
		assert.Equal(t, "true", cr.Labels[ar.label])
		assert.NotEmpty(t, cr.Rules, "ClusterRole %s should have rules", name)
	}
}

func TestReconcileAggregationClusterRoles_Updates(t *testing.T) {
	revision := "test-revision"
	existing := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   aggregationClusterRoleName(revision, "admin"),
			Labels: map[string]string{"old-label": "old-value"},
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"old"}, Resources: []string{"old"}, Verbs: []string{"get"}},
		},
	}
	cl := newFakeRBACClient(existing)

	err := reconcileAggregationClusterRoles(context.Background(), cl, newTestCRDManager(), revision)
	require.NoError(t, err)

	cr := &rbacv1.ClusterRole{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: aggregationClusterRoleName(revision, "admin")}, cr)
	require.NoError(t, err)

	assert.Equal(t, "true", cr.Labels["rbac.authorization.k8s.io/aggregate-to-admin"])
	assert.Equal(t, managedByValue, cr.Labels[managedByLabelKey])
	assert.NotContains(t, cr.Labels, "old-label")
	assert.NotEmpty(t, cr.Rules)
}

func TestDeleteAggregationClusterRoles(t *testing.T) {
	revision := "test-revision"
	cl := newFakeRBACClient()

	err := reconcileAggregationClusterRoles(context.Background(), cl, newTestCRDManager(), revision)
	require.NoError(t, err)

	err = deleteAggregationClusterRoles(context.Background(), cl, revision)
	require.NoError(t, err)

	for _, ar := range aggregationRoles {
		name := aggregationClusterRoleName(revision, ar.suffix)
		cr := &rbacv1.ClusterRole{}
		err := cl.Get(context.Background(), client.ObjectKey{Name: name}, cr)
		assert.Error(t, err, "ClusterRole %s should be deleted", name)
	}
}

func TestDeleteAggregationClusterRoles_NotFound(t *testing.T) {
	cl := newFakeRBACClient()
	err := deleteAggregationClusterRoles(context.Background(), cl, "nonexistent")
	require.NoError(t, err)
}

func TestAggregationClusterRoleName(t *testing.T) {
	assert.Equal(t, "istio-crd-admin-my-rev", aggregationClusterRoleName("my-rev", "admin"))
	assert.Equal(t, "istio-crd-edit-my-rev", aggregationClusterRoleName("my-rev", "edit"))
	assert.Equal(t, "istio-crd-view-my-rev", aggregationClusterRoleName("my-rev", "view"))
}

func TestRulesFromCRDNames(t *testing.T) {
	crdNames := []string{
		"telemetries.telemetry.istio.io",
		"virtualservices.networking.istio.io",
		"gateways.networking.istio.io",
		"authorizationpolicies.security.istio.io",
	}
	verbs := []string{"get", "list", "watch"}

	rules := rulesFromCRDNames(crdNames, verbs)

	assert.Len(t, rules, 3, "should have 3 rules (one per API group)")

	groupMap := make(map[string]rbacv1.PolicyRule)
	for _, r := range rules {
		groupMap[r.APIGroups[0]] = r
	}

	assert.Contains(t, groupMap["telemetry.istio.io"].Resources, "telemetries")
	assert.Contains(t, groupMap["networking.istio.io"].Resources, "virtualservices")
	assert.Contains(t, groupMap["networking.istio.io"].Resources, "gateways")
	assert.Contains(t, groupMap["security.istio.io"].Resources, "authorizationpolicies")

	for _, r := range rules {
		assert.Equal(t, verbs, r.Verbs)
	}
}

func TestRulesFromCRDNames_MatchesCRDFiles(t *testing.T) {
	cm := newTestCRDManager()
	crds, err := cm.loadCRDsMatching(Options{IncludeAllCRDs: true}, aggregatableCRD)
	require.NoError(t, err)
	require.NotEmpty(t, crds)

	var crdNames []string
	for _, crd := range crds {
		crdNames = append(crdNames, crd.Name)
	}

	rules := rulesFromCRDNames(crdNames, []string{"get"})

	expectedGroups := map[string]bool{
		"networking.istio.io": false,
		"security.istio.io":   false,
		"telemetry.istio.io":  false,
		"extensions.istio.io": false,
		"sailoperator.io":     false,
	}
	for _, rule := range rules {
		for _, group := range rule.APIGroups {
			if _, ok := expectedGroups[group]; ok {
				expectedGroups[group] = true
			}
		}
	}
	for group, found := range expectedGroups {
		assert.True(t, found, "rules should cover API group %s", group)
	}
}

func TestAggregationRoles_ViewHasReadOnlyVerbs(t *testing.T) {
	for _, ar := range aggregationRoles {
		if ar.suffix == "view" {
			assert.Equal(t, []string{"get", "list", "watch"}, ar.verbs)
		}
	}
}

func TestAggregationRoles_AdminHasAllVerbs(t *testing.T) {
	for _, ar := range aggregationRoles {
		if ar.suffix == "admin" {
			assert.Equal(t, []string{"*"}, ar.verbs)
		}
	}
}

func TestAggregationRoles_EditHasWriteOnlyVerbs(t *testing.T) {
	for _, ar := range aggregationRoles {
		if ar.suffix == "edit" {
			assert.Equal(t, []string{"create", "delete", "patch", "update"}, ar.verbs)
			assert.NotContains(t, ar.verbs, "get")
			assert.NotContains(t, ar.verbs, "list")
			assert.NotContains(t, ar.verbs, "watch")
		}
	}
}
