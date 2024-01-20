package webhook

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snapp-incubator/contour-admission-webhook/internal/cache"
	"github.com/snapp-incubator/contour-admission-webhook/internal/config"
	"github.com/snapp-incubator/contour-admission-webhook/pkg/utils"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestValidate(t *testing.T) {
	if err := config.InitializeConfig("../../hack/config.yaml"); err != nil {
		assert.FailNow(t, fmt.Sprintf("error reading the config file: %s", err.Error()))
	}

	config := config.GetConfig()

	cacheCleanUpInterval := time.Duration(config.Cache.CleanUpIntervalSecond) * time.Second
	cacheDuration := time.Duration(config.Cache.EntryTtlSecond) * time.Second
	validIngressClassNames := config.IngressClasses
	invalidIngressClassName := "invalid"
	allIngressClassNames := append(validIngressClassNames, invalidIngressClassName)

	t.Run("Should clean up expired keys from cache", func(t *testing.T) {

		testCache := cache.NewCache(1 * time.Second)
		testCache.Set("test/key",
			&types.NamespacedName{Namespace: "test", Name: "test"},
			time.Now().Add(1*time.Second).Unix(),
		)

		assert.True(t, testCache.KeyExists("test/key"))
		time.Sleep(3 * time.Second)
		assert.False(t, testCache.KeyExists("test/key"))
	})

	t.Run("Should not clean up keys without expiration time", func(t *testing.T) {

		testCache := cache.NewCache(1 * time.Second)
		testCache.Set("test/key",
			&types.NamespacedName{Namespace: "test", Name: "test"},
			0,
		)

		assert.True(t, testCache.KeyExists("test/key"))
		time.Sleep(3 * time.Second)
		assert.True(t, testCache.KeyExists("test/key"))
	})

	t.Run("Should return error indicating invalid \"Content-Type\" header", func(t *testing.T) {

		testCache := cache.NewCache(cacheCleanUpInterval)

		ah := &admissionHandler{
			cache:   testCache,
			handler: validateV1,
		}

		r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte{}))
		r.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()

		ah.ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should deny the admission request - CREATE operation with invalid ingressClassName", func(t *testing.T) {
		var admissionRequestJSON = `
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
							"ingressClassName": "%s",
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
								"fqdn": "%s"
							}
						}
					},
					"oldObject": null,
					"dryRun": false
				}
			}
		`

		ingressClassName := invalidIngressClassName
		fqdn := "test.local"

		localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, ingressClassName, fqdn)

		testCache := cache.NewCache(cacheCleanUpInterval)

		ah := &admissionHandler{
			cache:   testCache,
			handler: validateV1,
		}

		admissionReviewRequest := &admissionv1.AdmissionReview{}
		admissionReviewResponse := &admissionv1.AdmissionReview{}

		r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
		r.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		ah.ServeHTTP(w, r)

		assert.Equal(t, nil, json.Unmarshal([]byte(localAdmissionRequestJSON), admissionReviewRequest))
		assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, admissionReviewResponse.Response.Allowed)
		assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
	})

	t.Run("Should allow the admission request and add a cache entry for the requested FQDN in the map associated with the ingressClassName - CREATE operation with valid ingressClassName", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"oldObject": null,
						"dryRun": false
					}
				}
			`

		fqdn := "test.local"

		for _, ingressClassName := range validIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, ingressClassName, fqdn)

			cacheKey := utils.GenerateCacheKey(ingressClassName, fqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAdded := testCache.KeyExists(cacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(localAdmissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAdded)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - CREATE operation with valid ingressClassName", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"oldObject": null,
						"dryRun": true
					}
				}
			`

		fqdn := "test.local"

		for _, ingressClassName := range validIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, ingressClassName, fqdn)

			cacheKey := utils.GenerateCacheKey(ingressClassName, fqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAdded := testCache.KeyExists(cacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(localAdmissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.False(t, isFqdnAdded)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should deny the admission request because of the acquired FQDN - CREATE operation with valid ingressClassName", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"oldObject": null,
						"dryRun": false
					}
				}
			`

		fqdn := "test.local"

		for _, ingressClassName := range validIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, ingressClassName, fqdn)

			cacheKey := utils.GenerateCacheKey(ingressClassName, fqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)
			testCache.Set(cacheKey,
				&types.NamespacedName{Namespace: "test", Name: "test"},
				time.Now().Add(cacheDuration).Unix(),
			)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAcquired := testCache.KeyExists(cacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(localAdmissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAcquired)
			assert.False(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should allow the admission request and delete the cache entry for the old FQDN if any found and add a new one for the requested FQDN - UPDATE operation with new ingressClassName being valid and old ingressClassName being invalid", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		validNewIngressClassNames := validIngressClassNames
		invalidOldIngressClassName := invalidIngressClassName
		newFqdn := "new.test.local"
		oldFqdn := "old.test.local"

		for _, validNewIngressClassName := range validNewIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, newFqdn, invalidOldIngressClassName, oldFqdn)

			cacheKey := utils.GenerateCacheKey(validNewIngressClassName, newFqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAdded := testCache.KeyExists(cacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAdded)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should allow the admission request and retain the cache entry for the old FQDN and add a new one for the requested FQDN - UPDATE operation with both ingressClassNames being valid", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		newFqdn := "new.test.local"
		oldFqdn := "old.test.local"

		for _, validNewIngressClassName := range validIngressClassNames {
			for _, validOldIngressClassName := range validIngressClassNames {
				localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, newFqdn, validOldIngressClassName, oldFqdn)

				newCacheKey := utils.GenerateCacheKey(validNewIngressClassName, newFqdn)
				oldCacheKey := utils.GenerateCacheKey(validOldIngressClassName, oldFqdn)

				testCache := cache.NewCache(cacheCleanUpInterval)
				testCache.Set(oldCacheKey,
					&types.NamespacedName{Namespace: "test", Name: "test"},
					time.Now().Add(cacheDuration).Unix(),
				)

				ah := &admissionHandler{
					cache:   testCache,
					handler: validateV1,
				}

				admissionReviewRequest := &admissionv1.AdmissionReview{}
				admissionReviewResponse := &admissionv1.AdmissionReview{}

				r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
				r.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()

				ah.ServeHTTP(w, r)

				isFqdnAcquired := testCache.KeyExists(oldCacheKey)
				isFqdnAdded := testCache.KeyExists(newCacheKey)

				assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
				assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
				assert.Equal(t, http.StatusOK, w.Code)
				assert.True(t, isFqdnAcquired)
				assert.True(t, isFqdnAdded)
				assert.True(t, admissionReviewResponse.Response.Allowed)
				assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
			}
		}
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - UPDATE operation with new ingressClassName being valid and old ingressClassName being invalid", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": true
					}
				}
			`

		validNewIngressClassNames := validIngressClassNames
		invalidOldIngressClassName := invalidIngressClassName
		newFqdn := "new.test.local"
		oldFqdn := "old.test.local"

		for _, validNewIngressClassName := range validNewIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, newFqdn, invalidOldIngressClassName, oldFqdn)

			newCacheKey := utils.GenerateCacheKey(validNewIngressClassName, newFqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAdded := testCache.KeyExists(newCacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.False(t, isFqdnAdded)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - UPDATE operation with both ingressClassNames being valid", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": true
					}
				}
			`

		validNewIngressClassNames := validIngressClassNames
		newFqdn := "new.test.local"
		oldFqdn := "old.test.local"

		for _, validNewIngressClassName := range validNewIngressClassNames {
			for _, validOldIngressClassName := range validNewIngressClassNames {
				localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, newFqdn, validOldIngressClassName, oldFqdn)

				newCacheKey := utils.GenerateCacheKey(validNewIngressClassName, newFqdn)
				oldCacheKey := utils.GenerateCacheKey(validOldIngressClassName, oldFqdn)

				testCache := cache.NewCache(cacheCleanUpInterval)
				testCache.Set(oldCacheKey,
					&types.NamespacedName{Namespace: "test", Name: "test"},
					time.Now().Add(cacheDuration).Unix(),
				)

				ah := &admissionHandler{
					cache:   testCache,
					handler: validateV1,
				}

				admissionReviewRequest := &admissionv1.AdmissionReview{}
				admissionReviewResponse := &admissionv1.AdmissionReview{}

				r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
				r.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()

				ah.ServeHTTP(w, r)

				isFqdnAcquired := testCache.KeyExists(oldCacheKey)
				isFqdnAdded := testCache.KeyExists(newCacheKey)

				assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
				assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
				assert.Equal(t, http.StatusOK, w.Code)
				assert.True(t, isFqdnAcquired)
				assert.False(t, isFqdnAdded)
				assert.True(t, admissionReviewResponse.Response.Allowed)
				assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
			}
		}
	})

	t.Run("Should allow the admission request and delete the cache entry for the FQDN if any found and add a new one because the ingressClassName is changed (FQDN is not changed) - UPDATE operation with new ingressClassName being valid and old ingressClassName being invalid", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		validNewIngressClassNames := validIngressClassNames
		invalidOldIngressClassName := invalidIngressClassName
		sameFqdn := "same.test.local"

		for _, validNewIngressClassName := range validNewIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, sameFqdn, invalidOldIngressClassName, sameFqdn)

			newCacheKey := utils.GenerateCacheKey(validNewIngressClassName, sameFqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAdded := testCache.KeyExists(newCacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAdded)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should allow the admission request and retain the cache entry for the FQDN and add a new one because the ingressClassName is changed (FQDN is not changed) - UPDATE operation with both ingressClassNames being valid", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		validNewIngressClassNames := validIngressClassNames
		sameFqdn := "same.test.local"

		for _, validNewIngressClassName := range validNewIngressClassNames {
			for _, validOldIngressClassName := range validNewIngressClassNames {
				if validNewIngressClassName == validOldIngressClassName {
					continue
				}

				localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, sameFqdn, validOldIngressClassName, sameFqdn)

				newCacheKey := utils.GenerateCacheKey(validNewIngressClassName, sameFqdn)
				oldCacheKey := utils.GenerateCacheKey(validOldIngressClassName, sameFqdn)

				testCache := cache.NewCache(cacheCleanUpInterval)
				testCache.Set(oldCacheKey,
					&types.NamespacedName{Namespace: "test", Name: "test"},
					time.Now().Add(cacheDuration).Unix(),
				)

				ah := &admissionHandler{
					cache:   testCache,
					handler: validateV1,
				}

				admissionReviewRequest := &admissionv1.AdmissionReview{}
				admissionReviewResponse := &admissionv1.AdmissionReview{}

				r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
				r.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()

				ah.ServeHTTP(w, r)

				isFqdnAcquired := testCache.KeyExists(oldCacheKey)
				isFqdnAdded := testCache.KeyExists(newCacheKey)

				assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
				assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
				assert.Equal(t, http.StatusOK, w.Code)
				assert.True(t, isFqdnAcquired)
				assert.True(t, isFqdnAdded)
				assert.True(t, admissionReviewResponse.Response.Allowed)
				assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
			}
		}
	})

	t.Run("Should allow the admission request and not alter the cache because neither the ingressClassName nor the FQDN is changed - UPDATE operation", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		sameFqdn := "same.test.local"

		for _, validIngressClassName := range validIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validIngressClassName, sameFqdn, validIngressClassName, sameFqdn)

			newCacheKey := utils.GenerateCacheKey(validIngressClassName, sameFqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)
			testCache.Set(newCacheKey,
				&types.NamespacedName{Namespace: "test", Name: "test"},
				time.Now().Add(cacheDuration).Unix(),
			)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAcquired := testCache.KeyExists(newCacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAcquired)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should deny the admission request because of the acquired new FQDN and retain the cache entry for the old FQDN - UPDATE operation", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		validNewIngressClassNames := validIngressClassNames
		newFqdn := "new.test.local"
		oldFqdn := "old.test.local"

		for _, validNewIngressClassName := range validNewIngressClassNames {
			for _, validOldIngressClassName := range validNewIngressClassNames {

				localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validNewIngressClassName, newFqdn, validOldIngressClassName, oldFqdn)

				newCacheKey := utils.GenerateCacheKey(validNewIngressClassName, newFqdn)
				oldCacheKey := utils.GenerateCacheKey(validOldIngressClassName, oldFqdn)

				testCache := cache.NewCache(cacheCleanUpInterval)
				testCache.Set(oldCacheKey,
					&types.NamespacedName{Namespace: "test", Name: "test"},
					time.Now().Add(cacheDuration).Unix(),
				)
				testCache.Set(newCacheKey,
					&types.NamespacedName{Namespace: "test", Name: "test"},
					time.Now().Add(cacheDuration).Unix(),
				)

				ah := &admissionHandler{
					cache:   testCache,
					handler: validateV1,
				}

				admissionReviewRequest := &admissionv1.AdmissionReview{}
				admissionReviewResponse := &admissionv1.AdmissionReview{}

				r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
				r.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()

				ah.ServeHTTP(w, r)

				isFqdnRetained := testCache.KeyExists(oldCacheKey)
				isRequestedFqdnRetained := testCache.KeyExists(newCacheKey)

				assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
				assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
				assert.Equal(t, http.StatusOK, w.Code)
				assert.True(t, isFqdnRetained)
				assert.True(t, isRequestedFqdnRetained)
				assert.False(t, admissionReviewResponse.Response.Allowed)
				assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
			}
		}
	})

	t.Run("Should allow the admission request and retain the cache entry for the requested FQDN if any - DELETE operation", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": false
					}
				}
			`

		fqdn := "test.local"

		for _, ingressClassName := range allIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, ingressClassName, fqdn)

			newCacheKey := utils.GenerateCacheKey(ingressClassName, fqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)

			testCache.Set(newCacheKey,
				&types.NamespacedName{Namespace: "test", Name: "test"},
				time.Now().Add(cacheDuration).Unix(),
			)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAcquired := testCache.KeyExists(newCacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAcquired)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})

	t.Run("Should allow the admission request and not alter the cache for dry-run requests - DELETE operation", func(t *testing.T) {
		var admissionRequestJSON = `
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
								"ingressClassName": "%s",
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
									"fqdn": "%s"
								}
							}
						},
						"dryRun": true
					}
				}
			`

		fqdn := "test.local"

		for _, validIngressClassName := range validIngressClassNames {
			localAdmissionRequestJSON := fmt.Sprintf(admissionRequestJSON, validIngressClassName, fqdn)

			newCacheKey := utils.GenerateCacheKey(validIngressClassName, fqdn)

			testCache := cache.NewCache(cacheCleanUpInterval)
			testCache.Set(newCacheKey,
				&types.NamespacedName{Namespace: "test", Name: "test"},
				time.Now().Add(cacheDuration).Unix(),
			)

			ah := &admissionHandler{
				cache:   testCache,
				handler: validateV1,
			}

			admissionReviewRequest := &admissionv1.AdmissionReview{}
			admissionReviewResponse := &admissionv1.AdmissionReview{}

			r := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewReader([]byte(localAdmissionRequestJSON)))
			r.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			ah.ServeHTTP(w, r)

			isFqdnAcquired := testCache.KeyExists(newCacheKey)

			assert.Equal(t, nil, json.Unmarshal([]byte(admissionRequestJSON), admissionReviewRequest))
			assert.Equal(t, nil, json.Unmarshal(w.Body.Bytes(), admissionReviewResponse))
			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, isFqdnAcquired)
			assert.True(t, admissionReviewResponse.Response.Allowed)
			assert.Equal(t, admissionReviewRequest.Request.UID, admissionReviewResponse.Response.UID)
		}
	})
}
