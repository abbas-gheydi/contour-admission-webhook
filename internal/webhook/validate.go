package webhook

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type checker interface {
	check(cr *checkRequest) (*admissionv1.AdmissionResponse, error)
	setNext(checker)
}

//nolint:varnamelen
func validateV1(ar admissionv1.AdmissionReview, cache *cache.Cache) (*admissionv1.AdmissionResponse, error) {
	contourv1HttpproxyResource := metav1.GroupVersionResource{Group: "projectcontour.io", Version: "v1", Resource: "httpproxies"}

	if ar.Request.Resource != contourv1HttpproxyResource {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Resource being requested must be %s", contourv1HttpproxyResource))
	}

	httpproxy := &contourv1.HTTPProxy{}
	httpproxyOld := &contourv1.HTTPProxy{}

	if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, httpproxy); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Resource being requested could not be deserialized: %s", err.Error()))
	}

	if _, _, err := deserializer.Decode(ar.Request.OldObject.Raw, nil, httpproxyOld); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Resource being requested could not be deserialized: %s", err.Error()))
	}

	cr := &checkRequest{
		newObj: httpproxy,
		oldObj: httpproxyOld,
		dryRun: ar.Request.DryRun,
		cache:  cache,
	}

	switch ar.Request.Operation {
	case admissionv1.Create:
		cfoc := checkFqdnOnCreate{}

		response, err := cfoc.check(cr)

		return response, err

	case admissionv1.Update:
		cfou := checkFqdnOnUpdate{}

		response, err := cfou.check(cr)

		return response, err

	case admissionv1.Delete:
		cfod := checkFqdnOnDelete{}

		response, err := cfod.check(cr)

		return response, err
	}

	return nil, echo.NewHTTPError(http.StatusBadRequest,
		"Operation being performed on the requested resource must be one of CREATE, UPDATE or DELETE")
}
