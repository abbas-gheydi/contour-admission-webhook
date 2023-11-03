package webhook

import (
	"net/http"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type checkRequest struct {
	new   *contourv1.HTTPProxy
	old   *contourv1.HTTPProxy
	cache *cache.Cache
}

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
func (cfoc checkFqdnOnCreate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, error) {
	cr.cache.Mu.Lock()
	defer cr.cache.Mu.Unlock()

	if cr.new.Spec.VirtualHost == nil {
		if cfoc.next != nil {
			return cfoc.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	fqdn := cr.new.Spec.VirtualHost.Fqdn

	_, acquired := cr.cache.FqdnMap[fqdn]
	if acquired {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusForbidden,
				Message: "HTTPProxy's fqdn is already acquired by another HTTPProxy object",
			}}, nil
	}

	cr.cache.FqdnMap[fqdn] = &types.NamespacedName{Namespace: cr.new.Namespace, Name: cr.new.Name}

	if cfoc.next != nil {
		return cfoc.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfoc *checkFqdnOnCreate) setNext(c checker) {
	cfoc.next = c
}

//nolint:varnamelen
func (cfou checkFqdnOnUpdate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, error) {
	cr.cache.Mu.Lock()
	defer cr.cache.Mu.Unlock()

	//nolint:nestif
	if cr.new.Spec.VirtualHost == nil && cr.old.Spec.VirtualHost == nil {
		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	} else if cr.new.Spec.VirtualHost == nil && cr.old.Spec.VirtualHost != nil {
		oldFqdn := cr.old.Spec.VirtualHost.Fqdn

		delete(cr.cache.FqdnMap, oldFqdn)

		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	} else if cr.new.Spec.VirtualHost != nil && cr.old.Spec.VirtualHost == nil {
		fqdn := cr.new.Spec.VirtualHost.Fqdn

		_, acquired := cr.cache.FqdnMap[fqdn]
		if acquired {
			return &admissionv1.AdmissionResponse{Allowed: false,
				Result: &metav1.Status{
					// http code and message returned to the user
					Code:    http.StatusForbidden,
					Message: "HTTPProxy's fqdn is already acquired by another HTTPProxy object",
				}}, nil
		}

		cr.cache.FqdnMap[fqdn] = &types.NamespacedName{Namespace: cr.new.Namespace, Name: cr.new.Name}

		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	fqdn := cr.new.Spec.VirtualHost.Fqdn
	oldFqdn := cr.old.Spec.VirtualHost.Fqdn

	if fqdn == oldFqdn {
		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	_, acquired := cr.cache.FqdnMap[fqdn]
	if acquired {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusForbidden,
				Message: "HTTPProxy's fqdn is already acquired by another HTTPProxy object",
			}}, nil
	}

	cr.cache.FqdnMap[fqdn] = &types.NamespacedName{Namespace: cr.new.Namespace, Name: cr.new.Name}

	delete(cr.cache.FqdnMap, oldFqdn)

	if cfou.next != nil {
		return cfou.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfou *checkFqdnOnUpdate) setNext(c checker) {
	cfou.next = c
}

//nolint:varnamelen
func (cfod checkFqdnOnDelete) check(cr *checkRequest) (*admissionv1.AdmissionResponse, error) {
	cr.cache.Mu.Lock()
	defer cr.cache.Mu.Unlock()

	if cr.old.Spec.VirtualHost == nil {
		if cfod.next != nil {
			return cfod.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	oldFqdn := cr.old.Spec.VirtualHost.Fqdn

	delete(cr.cache.FqdnMap, oldFqdn)

	if cfod.next != nil {
		return cfod.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfod *checkFqdnOnDelete) setNext(c checker) {
	cfod.next = c
}
