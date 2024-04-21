package webhook

import "fmt"

type mutateIdleConnectionTimeout struct {
	next mutator
}

//nolint:varnamelen
func (mict mutateIdleConnectionTimeout) mutate(mr *mutateRequest) {
	//nolint:nestif
	if mr.newObj.Spec.TCPProxy == nil && len(mr.newObj.Spec.Routes) > 0 {
		var mutated bool

		for index, route := range mr.newObj.Spec.Routes {
			if route.TimeoutPolicy != nil {
				if route.TimeoutPolicy.IdleConnection != "" {
					continue
				}

				mr.mutations = append(mr.mutations,
					map[string]interface{}{
						"op":    "add",
						"path":  fmt.Sprintf("/spec/routes/%d/timeoutPolicy/idleConnection", index),
						"value": "15s",
					})

				mutated = true
			} else {
				mr.mutations = append(mr.mutations,
					map[string]interface{}{
						"op":   "add",
						"path": fmt.Sprintf("/spec/routes/%d/timeoutPolicy", index),
						"value": map[string]string{
							"idleConnection": "15s",
						},
					})

				mutated = true
			}
		}

		if mutated {
			if mr.newObj.Annotations != nil {
				mr.mutations = append(mr.mutations,
					map[string]interface{}{
						"op":    "add",
						"path":  "/metadata/annotations/policies.network.snappcloud.io~1added-timeoutPolicy-idleConnection",
						"value": "",
					})
			} else {
				mr.mutations = append(mr.mutations,
					map[string]interface{}{
						"op":   "add",
						"path": "/metadata/annotations",
						"value": map[string]string{
							"policies.network.snappcloud.io/added-timeoutPolicy-idleConnection": "",
						},
					})
			}
		}
	}

	if mict.next != nil {
		mict.next.mutate(mr)
	}
}

func (mict *mutateIdleConnectionTimeout) setNext(m mutator) {
	mict.next = m
}
