package webhook

import (
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	scheme       = runtime.NewScheme()
	codecFactory = serializer.NewCodecFactory(scheme)
	deserializer = codecFactory.UniversalDeserializer()
)

type admitV1Func func(admissionv1.AdmissionReview, *cache.Cache) (*admissionv1.AdmissionResponse, error)

func init() {
	utilruntime.Must(admissionv1.AddToScheme(scheme))
	utilruntime.Must(contourv1.AddToScheme(scheme))
}

// serve handles the http portion of a request prior to handing to an admit function.
//
//nolint:varnamelen
func serve(c echo.Context, cache *cache.Cache, admitV1 admitV1Func) error {
	var body []byte

	if c.Request().Body != nil {
		data, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError,
				fmt.Sprintf("Could not read request body: %s", err.Error()))
		}

		body = data
	}

	contentType := c.Request().Header.Get("Content-Type")
	if contentType != echo.MIMEApplicationJSON {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Content-Type header is %s, must be application/json", contentType))
	}

	var responseObj runtime.Object

	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("HTTP request body could not be decoded: %s", err.Error()))
	}

	admissionReviewRequest, ok := obj.(*admissionv1.AdmissionReview)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Expected v1.AdmissionReview object but got: %T object", obj))
	}

	// Can not use the already declared err interface
	// Impossible comparison of interface value with untyped nil
	// https://staticcheck.dev/docs/checks#SA4023
	admitResponse, admitError := admitV1(*admissionReviewRequest, cache)
	if admitError != nil {
		return admitError
	}

	admissionReviewResponse := &admissionv1.AdmissionReview{}
	admissionReviewResponse.SetGroupVersionKind(*gvk)
	admissionReviewResponse.Response = admitResponse
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID
	responseObj = admissionReviewResponse

	return c.JSON(http.StatusOK, responseObj)
}

func Setup(address, tlsCertPath, tlsKeyPath string, cache *cache.Cache) {
	//nolint:varnamelen
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/v1/validate", func(c echo.Context) error {
		return serve(c, cache, validateV1)
	})
	e.GET("/readyz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	e.Logger.Fatal(e.StartTLS(address, tlsCertPath, tlsKeyPath))
}
