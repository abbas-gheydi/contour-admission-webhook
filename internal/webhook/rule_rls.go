package webhook

import (
	"github.com/snapp-incubator/contour-global-ratelimit-operator/pkg/rlsparser"
	admissionv1 "k8s.io/api/admission/v1"
)

type rlsValidator struct {
	next checker
}

func (e *rlsValidator) check(checkrequest *checkRequest) (*admissionv1.AdmissionResponse, *httpErr) {
	// check if there is any error in parsing rls configs in HTTPProxy Object
	_, _, err := rlsparser.ParseGlobalRateLimit(checkrequest.newObj)
	if err != nil {
		return acceptWithWarning(err.Error())
	}

	if e.next != nil {
		return e.next.check(checkrequest)
	}

	return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

func (e *rlsValidator) setNext(c checker) {
	e.next = c
}
