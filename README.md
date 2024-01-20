# contour-admission-webhook

## Introduction
`contour-admission-webhook` is a Kubernetes admission webhook server designed to assist in the validation of HTTPProxy objects. The primary objective of this webhook is to prevent the existence of duplicate Fully Qualified Domain Names (FQDNs) across HTTPProxy objects, but it also offers the flexibility to be expanded with multiple chained rules.
When duplicates occur, all the related HTTPProxy objects turn invalid, leading to HTTP 404 errors because the associated configurations are removed from the Envoy proxy.

## How It Works

### Overview:
The `contour-admission-webhook` server relies on an in-memory cache, initialized at startup, to maintain a map of FQDNs to their respective owner references before it begins processing requests. Among the rules within the rule chain, the FQDN validation rule specifically utilizes this in-memory cache.

### IngressClassName Validation Flow:
- CREATE/UPDATE Operations:

  When creating or updating an HTTPProxy object, it is a requirement that the value specified for `spec.ingressClassName` must correspond to one of the configured ingressClassNames. Additionally, the `spec.ingressClassName` cannot be an empty string.

- DELETE Operations:

  Upon executing a DELETE operation, the validation for `spec.ingressClassName` is not enforced. The deletion process does not involve the validation of the `spec.ingressClassName` value, allowing for the removal of the corresponding HTTPProxy object without considering this specific parameter.

### FQDN Validation Flow:
- CREATE/UPDATE Operations:

  When attempting to create or update an HTTPProxy object, the webhook first checks the requested FQDN against the built-in cache. If a match is found, the webhook operation is declined, accompanied by a message detailing the reason.
  If no match is found, the FQDN is added to the cache with a TTL, and the operation gets approved. A TTL mechanism is implemented to prevent the persistence of invalid states in the cache, ensuring timely removal upon rejection in other validating webhooks. After persisting in the state storage (etcd), the TTL is removed by a Kubernetes controller which watches HTTPProxy objects.
  For UPDATE operations specifically, any previous FQDN associated with the object is removed from the cache by a Kubernetes controller. This cleanup occurs after the object is persisted in the state storage (etcd).

- DELETE Operations:

  Upon executing a DELETE operation, the operation gets approved and the corresponding FQDN entry is purged from the cache after the object is persisted in the state storage (etcd).

<!-- ## Getting Started -->

## Contributing Guide
We appreciate your interest in contributing to our `contour-admission-webhook` project! This guide will walk you through the process of building upon our foundation, crafted using the Chain of Responsibility pattern.

### Rule Chain
The rule chain contains rules that are executed in the order set to validate the request. This pattern allows us to have great isolation between each rule. It also gives us the possibility to re-order the rules if what the webhook is supposed to do changes.

### Adding a New Validating Rule
1. Create a new rule file:
   
   Inside the `internal/webhook` directory, create a new Go file named after your rule, e.g. `rule_example.go`.

2. Implement the `checker` Interface:
    ```Go
    type checker interface {
	    check(cr *checkRequest) (*admissionv1.AdmissionResponse, error)
	    setNext(checker)
    }
    ```

1. Write your rule logic:

    ```Go
    type exampleRule struct {
        next Rule
    }

    func (e *exampleRule) check(cr *checkRequest) (*admissionv1.AdmissionResponse, error) {
        // Your rule logic here
        // ...

        if e.next != nil {
            return e.next.check(cr)
        }

        return &admissionv1.AdmissionResponse{Allowed: true}, nil
    }

    func (e *exampleRule) setNext(c checker) {
        e.next = c
    }
    ```

4. Add your rule to the chain:
   
   Modify the chain initialiser in include your rule.

## To Do

Below is a list of tasks that need attention. If you're contributing to this project or managing it, this section serves as a quick reference for ongoing and upcoming work.
- Add Helm chart
- Make cache key configurable
- Implement a mechanism to warm up the controller cache before launching the webhook server 
- Add E2E tests
