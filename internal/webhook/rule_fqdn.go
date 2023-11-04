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
	newObj *contourv1.HTTPProxy
	oldObj *contourv1.HTTPProxy
	dryRun *bool
	cache  *cache.Cache
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
	var dryRun bool

	if cr.dryRun != nil {
		dryRun = *cr.dryRun
	}

	cr.cache.Mu.Lock()
	defer cr.cache.Mu.Unlock()

	if cr.newObj.Spec.VirtualHost == nil {
		if cfoc.next != nil {
			return cfoc.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	fqdn := cr.newObj.Spec.VirtualHost.Fqdn

	_, acquired := cr.cache.FqdnMap[fqdn]
	if acquired {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusForbidden,
				Message: "HTTPProxy's fqdn is already acquired by another HTTPProxy object",
			}}, nil
	}

	if !dryRun {
		cr.cache.FqdnMap[fqdn] = &types.NamespacedName{Namespace: cr.newObj.Namespace, Name: cr.newObj.Name}
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
func (cfou checkFqdnOnUpdate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, error) {
	var dryRun bool

	if cr.dryRun != nil {
		dryRun = *cr.dryRun
	}

	cr.cache.Mu.Lock()
	defer cr.cache.Mu.Unlock()

	//nolint:nestif
	if cr.newObj.Spec.VirtualHost == nil && cr.oldObj.Spec.VirtualHost == nil {
		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	} else if cr.newObj.Spec.VirtualHost == nil && cr.oldObj.Spec.VirtualHost != nil {
		oldFqdn := cr.oldObj.Spec.VirtualHost.Fqdn

		if !dryRun {
			delete(cr.cache.FqdnMap, oldFqdn)
		}

		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	} else if cr.newObj.Spec.VirtualHost != nil && cr.oldObj.Spec.VirtualHost == nil {
		fqdn := cr.newObj.Spec.VirtualHost.Fqdn

		_, acquired := cr.cache.FqdnMap[fqdn]
		if acquired {
			return &admissionv1.AdmissionResponse{Allowed: false,
				Result: &metav1.Status{
					// http code and message returned to the user
					Code:    http.StatusForbidden,
					Message: "HTTPProxy's fqdn is already acquired by another HTTPProxy object",
				}}, nil
		}

		if !dryRun {
			cr.cache.FqdnMap[fqdn] = &types.NamespacedName{Namespace: cr.newObj.Namespace, Name: cr.newObj.Name}
		}

		if cfou.next != nil {
			return cfou.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	fqdn := cr.newObj.Spec.VirtualHost.Fqdn
	oldFqdn := cr.oldObj.Spec.VirtualHost.Fqdn

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

	if !dryRun {
		cr.cache.FqdnMap[fqdn] = &types.NamespacedName{Namespace: cr.newObj.Namespace, Name: cr.newObj.Name}
		delete(cr.cache.FqdnMap, oldFqdn)
	}

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
	var dryRun bool

	if cr.dryRun != nil {
		dryRun = *cr.dryRun
	}

	cr.cache.Mu.Lock()
	defer cr.cache.Mu.Unlock()

	if cr.oldObj.Spec.VirtualHost == nil {
		if cfod.next != nil {
			return cfod.next.check(cr)
		}

		return &admissionv1.AdmissionResponse{Allowed: true}, nil
	}

	oldFqdn := cr.oldObj.Spec.VirtualHost.Fqdn

	if !dryRun {
		delete(cr.cache.FqdnMap, oldFqdn)
	}

	if cfod.next != nil {
		return cfod.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cfod *checkFqdnOnDelete) setNext(c checker) {
	cfod.next = c
}
