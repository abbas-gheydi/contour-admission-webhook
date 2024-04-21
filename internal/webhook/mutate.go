package webhook

import (
	"fmt"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mutator interface {
	mutate(mr *mutateRequest)
	setNext(mutator)
}

type mutateRequest struct {
	newObj    *contourv1.HTTPProxy
	mutations []map[string]interface{}
}

//nolint:varnamelen
func mutateV1(ar admissionv1.AdmissionReview, _ *cache.Cache) (*admissionv1.AdmissionResponse, *httpErr) {
	contourv1HttpproxyResource := metav1.GroupVersionResource{Group: "projectcontour.io", Version: "v1", Resource: "httpproxies"}

	if ar.Request.Resource != contourv1HttpproxyResource {
		return nil, &httpErr{code: http.StatusBadRequest,
			message: fmt.Sprintf("requested resource must be %s", contourv1HttpproxyResource)}
	}

	httpproxy := &contourv1.HTTPProxy{}

	if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, httpproxy); err != nil {
		return nil, &httpErr{code: http.StatusInternalServerError,
			message: fmt.Sprintf("requested resource could not be deserialized: %s", err.Error())}
	}

	mr := &mutateRequest{
		newObj:    httpproxy,
		mutations: []map[string]interface{}{},
	}

	response := &admissionv1.AdmissionResponse{}
	response.Allowed = true

	switch ar.Request.Operation {
	case admissionv1.Create, admissionv1.Update:
		mict := mutateIdleConnectionTimeout{}

		mict.mutate(mr)

		patch, err := jsoniter.Marshal(mr.mutations)
		if err != nil {
			return nil, &httpErr{code: http.StatusInternalServerError,
				message: fmt.Sprintf("error encoding patch object to json: %s", err.Error())}
		}

		patchType := admissionv1.PatchTypeJSONPatch
		response.PatchType = &patchType
		response.Patch = patch

		return response, nil

	case admissionv1.Delete:
		return response, nil
	}

	return nil, &httpErr{code: http.StatusBadRequest,
		message: "operation being performed on the requested resource must be one of CREATE, UPDATE or DELETE"}
}
