package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestValidate(t *testing.T) {
	t.Run("Should return error indicating invalid \"Content-Type\" header", func(t *testing.T) {

		testCache := cache.NewCache()

		e := echo.New()

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte{}))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		assert.NotEqual(t, nil, err)
		assert.NotEqual(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Should allow the admission request and add a cache entry for the requested FQDN - CREATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "abb483ba-8193-4bef-9a39-245646e30506",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "CREATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "test.local"
							}
						}
					},
					"oldObject": null,
					"dryRun": false
				}
			}
		`)

		testCache := cache.NewCache()

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAdded := testCache.FqdnMap["test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.True(t, isFqdnAdded)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - CREATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "abb483ba-8193-4bef-9a39-245646e30506",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "CREATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "test.local"
							}
						}
					},
					"oldObject": null,
					"dryRun": true
				}
			}
		`)

		testCache := cache.NewCache()

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAdded := testCache.FqdnMap["test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.False(t, isFqdnAdded)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should deny the admission request because of the acquired FQDN - CREATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "abb483ba-8193-4bef-9a39-245646e30506",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "CREATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "test.local"
							}
						}
					},
					"oldObject": null,
					"dryRun": false
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAcquired := testCache.FqdnMap["test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.True(t, isFqdnAcquired)
		assert.False(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and delete the cache entry for the old FQDN and add a new one for the requested FQDN - UPDATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "529df94e-15df-47db-9959-07dab4a9effb",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "UPDATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:25:01Z",
							"generation": 2,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "new.test.local"
							}
						}
					},
					"oldObject": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "old.test.local"
							}
						}
					},
					"dryRun": false
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["old.test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAcquired := testCache.FqdnMap["old.test.local"]
		_, isFqdnAdded := testCache.FqdnMap["new.test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.False(t, isFqdnAcquired)
		assert.True(t, isFqdnAdded)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - UPDATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "529df94e-15df-47db-9959-07dab4a9effb",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "UPDATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:25:01Z",
							"generation": 2,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "new.test.local"
							}
						}
					},
					"oldObject": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "old.test.local"
							}
						}
					},
					"dryRun": true
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["old.test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAcquired := testCache.FqdnMap["old.test.local"]
		_, isFqdnAdded := testCache.FqdnMap["new.test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.True(t, isFqdnAcquired)
		assert.False(t, isFqdnAdded)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and retain the cache entry for the FQDN because the FQDN is not changed - UPDATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "529df94e-15df-47db-9959-07dab4a9effb",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "UPDATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:25:01Z",
							"generation": 2,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"h2"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "same.test.local"
							}
						}
					},
					"oldObject": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "same.test.local"
							}
						}
					},
					"dryRun": false
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["same.test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAcquired := testCache.FqdnMap["same.test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.True(t, isFqdnAcquired)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should deny the admission request because of the acquired new FQDN and retain the cache entry for the old FQDN - UPDATE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "529df94e-15df-47db-9959-07dab4a9effb",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "UPDATE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:25:01Z",
							"generation": 2,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "new.test.local"
							}
						}
					},
					"oldObject": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "4fbcd302-91b9-4680-b628-5144994ae612"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "old.test.local"
							}
						}
					},
					"dryRun": false
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["old.test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}
		testCache.FqdnMap["new.test.local"] = &types.NamespacedName{Namespace: "test-new", Name: "test-new"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnRetained := testCache.FqdnMap["old.test.local"]
		_, isRequestedFqdnRetained := testCache.FqdnMap["new.test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.True(t, isFqdnRetained)
		assert.True(t, isRequestedFqdnRetained)
		assert.False(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and delete the cache entry for the requested FQDN - DELETE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "833f7f5c-5df8-4942-8e2b-11d4c20f81d5",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "DELETE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": null,
					"oldObject": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "48929c85-4e00-4675-9f5c-bd58701879bc"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "test.local"
							}
						}
					},
					"dryRun": false
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAcquired := testCache.FqdnMap["test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.False(t, isFqdnAcquired)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - DELETE operation", func(t *testing.T) {
		var admissionRequestJSON = []byte(`
			{
				"kind": "AdmissionReview",
				"apiVersion": "admission.k8s.io/v1",
				"request": {
					"uid": "833f7f5c-5df8-4942-8e2b-11d4c20f81d5",
					"kind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"resource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"requestKind": {
						"group": "projectcontour.io",
						"version": "v1",
						"kind": "HTTPProxy"
					},
					"requestResource": {
						"group": "projectcontour.io",
						"version": "v1",
						"resource": "httpproxies"
					},
					"name": "test",
					"namespace": "test",
					"operation": "DELETE",
					"userInfo": {
						"username": "test",
						"groups": [
							"test",
							"system:authenticated"
						]
					},
					"object": null,
					"oldObject": {
						"apiVersion": "projectcontour.io/v1",
						"kind": "HTTPProxy",
						"metadata": {
							"annotations": {
								"k1": "v1"
							},
							"creationTimestamp": "2023-10-29T16:24:01Z",
							"generation": 1,
							"name": "test",
							"namespace": "test",
							"uid": "48929c85-4e00-4675-9f5c-bd58701879bc"
						},
						"spec": {
							"httpVersions": [
								"http/1.1"
							],
							"ingressClassName": "test",
							"routes": [
								{
									"conditions": [
										{
											"prefix": "/"
										}
									],
									"services": [
										{
											"name": "test",
											"port": 80
										}
									]
								}
							],
							"virtualhost": {
								"fqdn": "test.local"
							}
						}
					},
					"dryRun": true
				}
			}
		`)

		testCache := cache.NewCache()
		testCache.FqdnMap["test.local"] = &types.NamespacedName{Namespace: "test", Name: "test"}

		e := echo.New()

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		req := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader(admissionRequestJSON))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

		rec := httptest.NewRecorder()

		c := e.NewContext(req, rec)

		err := serve(c, testCache, validateV1)

		_, isFqdnAcquired := testCache.FqdnMap["test.local"]

		assert.Equal(t, nil, json.Unmarshal(admissionRequestJSON, admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(rec.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, nil, err)
		assert.True(t, isFqdnAcquired)
		assert.True(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})
}
