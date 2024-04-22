//go:build integration

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

package integration

import (
	"context"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("IstioRevision resource", Ordered, func() {
	const (
		revName        = "test-istiorevision"
		istioNamespace = "istiorevision-test"

		pilotImage = "sail-operator/test:latest"
	)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	revKey := client.ObjectKey{Name: revName}
	istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: istioNamespace}
	webhookKey := client.ObjectKey{Name: "istio-sidecar-injector-" + revName + "-" + istioNamespace}

	BeforeAll(func() {
		Step("Creating the Namespace to perform the tests")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespace to perform the tests")
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

		Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1alpha1.IstioRevision{}).Should(Succeed())
		Eventually(func(g Gomega) {
			list := &v1alpha1.IstioRevisionList{}
			g.Expect(k8sClient.List(ctx, list)).To(Succeed())
			g.Expect(list.Items).To(BeEmpty())
		}).Should(Succeed())
	})

	rev := &v1alpha1.IstioRevision{}

	Describe("validation", func() {
		AfterEach(func() {
			Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1alpha1.IstioRevision{}).Should(Succeed())
		})

		It("rejects an IstioRevision where spec.values.global.istioNamespace doesn't match spec.namespace", func() {
			rev = &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Version:   supportedversion.Default,
					Namespace: istioNamespace,
					Values: &v1alpha1.Values{
						Revision: revName,
						Global: &v1alpha1.GlobalConfig{
							IstioNamespace: "wrong-namespace",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Not(Succeed()))
		})

		It("rejects an IstioRevision where spec.values.revision doesn't match metadata.name (when name is not default)", func() {
			rev = &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Version:   supportedversion.Default,
					Namespace: istioNamespace,
					Values: &v1alpha1.Values{
						Revision: "is-not-" + revName,
						Global: &v1alpha1.GlobalConfig{
							IstioNamespace: istioNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Not(Succeed()))
		})

		It("rejects an IstioRevision where metadata.name is default and spec.values.revision isn't empty", func() {
			rev = &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Version:   supportedversion.Default,
					Namespace: istioNamespace,
					Values: &v1alpha1.Values{
						Revision: "default", // this must be rejected, because revision needs to be '' when metadata.name is 'default'
						Global: &v1alpha1.GlobalConfig{
							IstioNamespace: istioNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Not(Succeed()))
		})

		It("accepts an IstioRevision where metadata.name is default and spec.values.revision is empty", func() {
			rev = &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Version:   supportedversion.Default,
					Namespace: istioNamespace,
					Values: &v1alpha1.Values{
						Revision: "",
						Global: &v1alpha1.GlobalConfig{
							IstioNamespace: istioNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})
	})

	Describe("reconciles immediately after target namespace is created", func() {
		BeforeAll(func() {
			Step("Creating the IstioRevision resource without the namespace")
			rev = &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Version:   supportedversion.Default,
					Namespace: "nonexistent-namespace",
					Values: &v1alpha1.Values{
						Revision: revName,
						Global: &v1alpha1.GlobalConfig{
							IstioNamespace: "nonexistent-namespace",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			Expect(k8sClient.Delete(ctx, rev)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, kube.Key(revName), rev).Should(ReturnNotFoundError())
		})

		It("indicates in the status that the namespace doesn't exist", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
				g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))

				reconciled := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionReconciled)
				g.Expect(reconciled.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(reconciled.Reason).To(Equal(v1alpha1.IstioRevisionReasonReconcileError))
				g.Expect(reconciled.Message).To(ContainSubstring(`namespace "nonexistent-namespace" doesn't exist`))
			}).Should(Succeed())
		})

		When("the namespace is created", func() {
			BeforeAll(func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nonexistent-namespace",
					},
				}
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			})

			It("reconciles immediately", func() {
				Step("Checking if istiod is deployed immediately")
				istiod := &appsv1.Deployment{}
				istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: "nonexistent-namespace"}
				Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).WithTimeout(10 * time.Second).Should(Succeed())

				Step("Checking if the status is updated")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))
					reconciled := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionReconciled)
					g.Expect(reconciled.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})
		})
	})

	It("successfully reconciles the resource", func() {
		Step("Creating the custom resource")
		rev = &v1alpha1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: revName,
			},
			Spec: v1alpha1.IstioRevisionSpec{
				Version:   supportedversion.Default,
				Namespace: istioNamespace,
				Values: &v1alpha1.Values{
					Global: &v1alpha1.GlobalConfig{
						IstioNamespace: istioNamespace,
					},
					Revision: revName,
					Pilot: &v1alpha1.PilotConfig{
						Image: pilotImage,
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, rev)).To(Succeed())

		Step("Checking if the resource was successfully created")
		Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())

		istiod := &appsv1.Deployment{}
		Step("Checking if Deployment was successfully created in the reconciliation")
		Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).Should(Succeed())
		Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(rev)))

		Step("Checking if the status is updated")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
			g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))
		}).Should(Succeed())
	})

	When("istiod readiness changes", func() {
		It("updates the status of the IstioRevision resource", func() {
			By("setting the Ready condition status to true when istiod is ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
				istiod.Status.Replicas = 1
				istiod.Status.ReadyReplicas = 1
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					readyCondition := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionReady)
					g.Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())

				istiod.Status.ReadyReplicas = 0
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					readyCondition := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionReady)
					g.Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				}).Should(Succeed())
			})
		})
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiod := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      istiodKey.Name,
					Namespace: istiodKey.Namespace,
				},
			}
			Expect(k8sClient.Delete(ctx, istiod)).To(Succeed())

			Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).Should(Succeed())
			Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
			Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(rev)))
		})
	})

	When("an owned cluster-scoped resource is deleted", func() {
		It("recreates the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookKey.Name,
				},
			}
			Expect(k8sClient.Delete(ctx, webhook)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, webhookKey, webhook).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		istiod := &appsv1.Deployment{}
		var originalImage string

		BeforeAll(func() {
			Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
			originalImage = istiod.Spec.Template.Spec.Containers[0].Image

			istiod.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			Expect(k8sClient.Update(ctx, istiod)).To(Succeed())
		})

		It("reverts the owned resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
				g.Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(originalImage))
			}).Should(Succeed())
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		webhook := &admissionv1.MutatingWebhookConfiguration{}
		var origWebhooks []admissionv1.MutatingWebhook

		BeforeAll(func() {
			Expect(k8sClient.Get(ctx, webhookKey, webhook)).To(Succeed())
			origWebhooks = webhook.Webhooks

			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
		})

		It("reverts the owned resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, webhookKey, webhook)).To(Succeed())
				g.Expect(webhook.Webhooks).To(Equal(origWebhooks))
			}).Should(Succeed())
		})
	})

	It("supports concurrent deployment of two control planes", func() {
		rev2Name := revName + "2"
		rev2Key := client.ObjectKey{Name: rev2Name}
		istiod2Key := client.ObjectKey{Name: "istiod-" + rev2Name, Namespace: istioNamespace}

		Step("Creating the second IstioRevision instance")
		rev2 := &v1alpha1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: rev2Key.Name,
			},
			Spec: v1alpha1.IstioRevisionSpec{
				Version:   supportedversion.Default,
				Namespace: istioNamespace,
				Values: &v1alpha1.Values{
					Global: &v1alpha1.GlobalConfig{
						IstioNamespace: istioNamespace,
					},
					Revision: rev2Key.Name,
					Pilot: &v1alpha1.PilotConfig{
						Image: pilotImage,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, rev2)).To(Succeed())

		Step("Checking if the resource was successfully created")
		Eventually(k8sClient.Get).WithArguments(ctx, rev2Key, rev2).Should(Succeed())

		Step("Checking if the status is updated")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, rev2Key, rev2)).To(Succeed())
			g.Expect(rev2.Status.ObservedGeneration).To(Equal(rev2.ObjectMeta.Generation))
		}).Should(Succeed())

		Step("Checking if Deployment was successfully created in the reconciliation")
		istiod := &appsv1.Deployment{}
		Eventually(k8sClient.Get).WithArguments(ctx, istiod2Key, istiod).Should(Succeed())
		Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(rev2)))
	})
})
