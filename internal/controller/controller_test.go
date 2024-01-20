/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
)

// Constants for test configuration
const (
	defaultNamespace string = "default"
	defaultName      string = "dummy"
	finalizerString  string = "snappcloud.io/httpproxy-webhook-cache"
	waitDuration            = 1 * time.Second
)

// Main block for testing httpproxy fqdn cache controller
var _ = Describe("Testing httpproxy fqdn cache Controller", func() {
	Context("Testing reconcile loop functionality", Ordered, func() {
		// Utility function to get a sample httpproxy
		getSampleHttpproxy := func(name, namespace string) *contourv1.HTTPProxy {
			return &contourv1.HTTPProxy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: contourv1.HTTPProxySpec{
					IngressClassName: "test",
					VirtualHost: &contourv1.VirtualHost{
						Fqdn: "test.local",
					},
				},
			}
		}

		// Utility function to delete a httpproxy
		deleteHttpproxy := func(httpproxy *contourv1.HTTPProxy) {
			Expect(k8sClient.Delete(context.Background(), httpproxy)).To(Succeed())

			Eventually(func(g Gomega) {
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Namespace: httpproxy.Namespace,
					Name:      httpproxy.Name,
				}, &contourv1.HTTPProxy{})
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		}

		It("should not add a persisting cache entry for fqdn when httpproxy ingressClassName is invalid", func() {
			// Get a sample httpproxy in the default namespace
			httpproxyObj := getSampleHttpproxy(defaultName, defaultNamespace)
			httpproxyObj.Spec.IngressClassName = "invalid"

			// Create the httpproxy object and verify it succeeds
			Expect(k8sClient.Create(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Verify
			cacheKey := utils.GenerateCacheKey(httpproxyObj.Spec.IngressClassName, httpproxyObj.Spec.VirtualHost.Fqdn)
			Expect(cacheStore.KeyExists(cacheKey)).To(BeFalse())

			// Cleanup
			deleteHttpproxy(httpproxyObj)
		})

		It("should add a persisting cache entry for fqdn when a httpproxy object is created or updated", func() {
			// Get a sample httpproxy in the default namespace
			httpproxyObj := getSampleHttpproxy(defaultName, defaultNamespace)

			// Create the httpproxy object and verify it succeeds
			Expect(k8sClient.Create(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Verify
			cacheKey := utils.GenerateCacheKey(httpproxyObj.Spec.IngressClassName, httpproxyObj.Spec.VirtualHost.Fqdn)
			Expect(cacheStore.KeyExists(cacheKey)).To(BeTrue())
			Expect(*cacheStore.IsKeyPersisted(cacheKey)).To(BeTrue())

			// Cleanup
			deleteHttpproxy(httpproxyObj)
		})

		It("should delete the persisting cache entry for fqdn when a httpproxy object is deleted", func() {
			// Get a sample httpproxy in the default namespace
			httpproxyObj := getSampleHttpproxy(defaultName, defaultNamespace)

			// Create the httpproxy object and verify it succeeds
			Expect(k8sClient.Create(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Verify
			cacheKey := utils.GenerateCacheKey(httpproxyObj.Spec.IngressClassName, httpproxyObj.Spec.VirtualHost.Fqdn)
			Expect(cacheStore.KeyExists(cacheKey)).To(BeTrue())
			Expect(*cacheStore.IsKeyPersisted(cacheKey)).To(BeTrue())

			// Delete the httpproxy object and verify it succeeds
			Expect(k8sClient.Delete(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Verify
			Expect(cacheStore.KeyExists(cacheKey)).To(BeFalse())
		})

		It("should add finalizer string to httpproxy object when it is created or updated", func() {
			// Get a sample httpproxy in the default namespace
			httpproxyObj := getSampleHttpproxy(defaultName, defaultNamespace)

			// Create the httpproxy object and verify it succeeds
			Expect(k8sClient.Create(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Retrieve the httpproxy object, ensuring the finalizer string is present
			currentHttpproxyObj := contourv1.HTTPProxy{}
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: defaultName}, &currentHttpproxyObj)).To(Succeed())
			Expect(currentHttpproxyObj.ObjectMeta.Finalizers).To(ContainElement(finalizerString))

			// Cleanup
			deleteHttpproxy(httpproxyObj)
		})

		It("should delete finalizer string from httpproxy object when it is deleted", func() {
			// Get a sample httpproxy in the default namespace
			httpproxyObj := getSampleHttpproxy(defaultName, defaultNamespace)

			// Create the httpproxy object and verify it succeeds
			Expect(k8sClient.Create(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Retrieve the created httpproxy object and verify it succeeds
			Expect(k8sClient.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: defaultName}, &contourv1.HTTPProxy{})).To(Succeed())

			// Delete the httpproxy object and verify it succeeds
			Expect(k8sClient.Delete(context.Background(), httpproxyObj)).To(Succeed())

			// Wait for a specified duration to ensure the state is stable
			time.Sleep(waitDuration)

			// Verify the removal of finalizer string
			Expect(apierrors.IsNotFound(k8sClient.Get(context.Background(), types.NamespacedName{Namespace: defaultNamespace, Name: defaultName}, &contourv1.HTTPProxy{}))).To(BeTrue())
		})

	})
})
