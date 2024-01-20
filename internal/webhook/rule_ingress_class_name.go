package webhook

import (
	"net/http"

	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type checkIngressClassNameOnCreate struct {
	next checker
}

type checkIngressClassNameOnUpdate struct {
	next checker
}

type checkIngressClassNameOnDelete struct {
	next checker
}

//nolint:varnamelen
func (cicnoc checkIngressClassNameOnCreate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	newIngressClassName := utils.GetIngressClassName(cr.newObj)

	if newIngressClassName == "" {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusBadRequest,
				Message: "ingressClassName is not set",
			}}, nil
	}

	isNewIngressClassNameValid := utils.ValidateIngressClassName(newIngressClassName)

	if !isNewIngressClassNameValid {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusBadRequest,
				Message: "ingressClassName is not valid",
			}}, nil
	}

	cr.newIngressClass = &ingressClass{
		name:  newIngressClassName,
		valid: true,
	}

	if cicnoc.next != nil {
		return cicnoc.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cicnoc *checkIngressClassNameOnCreate) setNext(c checker) {
	cicnoc.next = c
}

//nolint:varnamelen
func (cicnou checkIngressClassNameOnUpdate) check(cr *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	newIngressClassName := utils.GetIngressClassName(cr.newObj)

	if newIngressClassName == "" {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusBadRequest,
				Message: "ingressClassName is not set",
			}}, nil
	}

	isNewIngressClassNameValid := utils.ValidateIngressClassName(newIngressClassName)

	if !isNewIngressClassNameValid {
		return &admissionv1.AdmissionResponse{Allowed: false,
			Result: &metav1.Status{
				// http code and message returned to the user
				Code:    http.StatusBadRequest,
				Message: "ingressClassName is not valid",
			}}, nil
	}

	oldIngressClassName := utils.GetIngressClassName(cr.oldObj)
	isOldIngressClassNameValid := utils.ValidateIngressClassName(oldIngressClassName)

	cr.newIngressClass = &ingressClass{
		name:  newIngressClassName,
		valid: true,
	}
	cr.oldIngressClass = &ingressClass{
		name:  oldIngressClassName,
		valid: isOldIngressClassNameValid,
	}

	if cicnou.next != nil {
		return cicnou.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cicnou *checkIngressClassNameOnUpdate) setNext(c checker) {
	cicnou.next = c
}

//nolint:varnamelen
func (cicnod checkIngressClassNameOnDelete) check(cr *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	oldIngressClassName := utils.GetIngressClassName(cr.oldObj)

	isOldIngressClassNameValid := utils.ValidateIngressClassName(oldIngressClassName)

	cr.oldIngressClass = &ingressClass{
		name:  oldIngressClassName,
		valid: isOldIngressClassNameValid,
	}

	if cicnod.next != nil {
		return cicnod.next.check(cr)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (cicnod *checkIngressClassNameOnDelete) setNext(c checker) {
	cicnod.next = c
}
