package webhook

import (
	"fmt"
	"net/http"
	"time"

	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type checkFqdnOnCreate struct {
	next checker
}

type checkFqdnOnUpdate struct {
	next checker
}

type checkFqdnOnDelete struct {
	next checker
}

//nolint:varnamelen
func (cfoc checkFqdnOnCreate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	if cr.newObj.Spec.VirtualHost == nil {
		if cfoc.next != nil {
			return cfoc.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	// The newIngressClass object should be initialized and populated by previous rules.
	// Also the rule must validate new ingressClassName and return an error if not valid, therefore
	// newIngressClass.valid must be always true.
	if cr.newIngressClass == nil {
		return nil, &httpErr{code: http.StatusInternalServerError,
			message: "ingressClass struct is nil"}
	}

	var dryRun bool

	if cr.dryRun != nil {
		dryRun = *cr.dryRun
	}

	// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
	// checking for zero value is not required as it's handled in the Kube API server before sending admission request to the webhook.
	fqdn := cr.newObj.Spec.VirtualHost.Fqdn

	cacheKey := utils.GenerateCacheKey(cr.newIngressClass.name, fqdn)

	if cr.cache.KeyExists(cacheKey) {
		ownerObj, _ := cr.cache.Get(cacheKey)

		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// The http code and message returned to the user
				Code: http.StatusForbidden,
				Message: fmt.Sprintf("fqdn is already acquired by another httpproxy object named %s in namespace %s",
					ownerObj.Name,
					ownerObj.Namespace),
			}}, nil
	}

	if !dryRun {
		cr.cache.Set(cacheKey,
			&types.NamespacedName{Namespace: cr.newObj.Namespace, Name: cr.newObj.Name},
			time.Now().Add(time.Duration(entryTtlSecond)*time.Second).Unix(),
		)
	}

	if cfoc.next != nil {
		return cfoc.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfoc *checkFqdnOnCreate) setNext(c checker) {
	cfoc.next = c
}

//nolint:varnamelen
func (cfou checkFqdnOnUpdate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	if cr.newObj.Spec.VirtualHost == nil && cr.oldObj.Spec.VirtualHost == nil {
		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	// The newIngressClass object should be initialized and populated by previous rules.
	// Also the rule must validate new ingressClassName and return an error if not valid, therefore
	// newIngressClass.valid must be always true.
	if cr.newIngressClass == nil {
		return nil, &httpErr{code: http.StatusInternalServerError,
			message: "ingressClass struct is nil"}
	}
	// The oldIngressClass object should be initialized and populated by previous rules.
	// Also the rule should validate old ingressClassName and specify it via oldIngressClass.valid field.
	if cr.oldIngressClass == nil {
		return nil, &httpErr{code: http.StatusInternalServerError,
			message: "ingressClass struct is nil"}
	}

	if cr.newObj.Spec.VirtualHost == nil && cr.oldObj.Spec.VirtualHost != nil {
		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	var dryRun bool

	if cr.dryRun != nil {
		dryRun = *cr.dryRun
	}

	newIngressClassName := cr.newIngressClass.name
	oldIngressClassName := cr.oldIngressClass.name

	if cr.newObj.Spec.VirtualHost != nil && cr.oldObj.Spec.VirtualHost == nil {
		// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
		// checking for zero value is not required as it's handled in the Kube API server before sending admission request to the webhook.
		newFqdn := cr.newObj.Spec.VirtualHost.Fqdn

		cacheKey := utils.GenerateCacheKey(newIngressClassName, newFqdn)

		if cr.cache.KeyExists(cacheKey) {
			ownerObj, _ := cr.cache.Get(cacheKey)

			return &admissionv1.AdmissionResponse{Allowed: false,
				Result: &metav1.Status{
					// The http code and message returned to the user
					Code: http.StatusForbidden,
					Message: fmt.Sprintf("fqdn is already acquired by another httpproxy object named %s in namespace %s",
						ownerObj.Name,
						ownerObj.Namespace),
				}}, nil
		}

		if !dryRun {
			cr.cache.Set(cacheKey,
				&types.NamespacedName{Namespace: cr.newObj.Namespace, Name: cr.newObj.Name},
				time.Now().Add(time.Duration(entryTtlSecond)*time.Second).Unix(),
			)
		}

		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	// The FQDN is always set and validated against pattern "^(\\*\\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$", so
	// checking for zero value is not required as it's handled in the Kube API server before sending admission request to the webhook.
	fqdn := cr.newObj.Spec.VirtualHost.Fqdn
	oldFqdn := cr.oldObj.Spec.VirtualHost.Fqdn

	if fqdn == oldFqdn && newIngressClassName == oldIngressClassName {
		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	newCacheKey := utils.GenerateCacheKey(newIngressClassName, fqdn)

	if cr.cache.KeyExists(newCacheKey) {
		ownerObj, _ := cr.cache.Get(newCacheKey)

		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// The http code and message returned to the user
				Code: http.StatusForbidden,
				Message: fmt.Sprintf("fqdn is already acquired by another httpproxy object named %s in namespace %s",
					ownerObj.Name,
					ownerObj.Namespace),
			}}, nil
	}

	if !dryRun {
		cr.cache.Set(newCacheKey,
			&types.NamespacedName{Namespace: cr.newObj.Namespace, Name: cr.newObj.Name},
			time.Now().Add(time.Duration(entryTtlSecond)*time.Second).Unix(),
		)
	}

	if cfou.next != nil {
		return cfou.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfou *checkFqdnOnUpdate) setNext(c checker) {
	cfou.next = c
}

func (cfod checkFqdnOnDelete) check(cr *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	if cfod.next != nil {
		return cfod.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfod *checkFqdnOnDelete) setNext(c checker) {
	cfod.next = c
}
