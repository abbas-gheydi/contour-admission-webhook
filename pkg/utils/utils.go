package utils

import (
	"fmt"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/snapp-incubator/contour-admission-webhook/internal/config"
)

var (
	validIngressClasses *[]string
)

func BoolPointer(b bool) *bool {
	return &b
}

func GenerateCacheKey(ingressClassName, fqdn string) string {
	return fmt.Sprintf("%s/%s", ingressClassName, fqdn)
}

func GetIngressClassName(httpproxy *contourv1.HTTPProxy) string {
	ingressClassName := httpproxy.Spec.IngressClassName

	// For backwards compatibility, when the `kubernetes.io/ingress.class` annotation is set,
	// it is given precedence over this field.
	annotation, found := httpproxy.Annotations["kubernetes.io/ingress.class"]
	if found {
		ingressClassName = annotation
	}

	return ingressClassName
}

func ValidateIngressClassName(ingressClassName string) bool {
	if validIngressClasses == nil {
		cfg := config.GetConfig()
		validIngressClasses = &cfg.IngressClasses
	}

	for _, ingressClass := range *validIngressClasses {
		if ingressClassName == ingressClass {
			return true
		}
	}

	return false
}
