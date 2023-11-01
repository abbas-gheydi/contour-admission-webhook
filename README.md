# contour-admission-webhook

## Introduction
`contour-admission-webhook` is a Kubernetes admission webhook server designed to assist in the validation of HTTPProxy objects. The primary objective of this webhook is to prevent the existence of duplicate Fully Qualified Domain Names (FQDNs) across HTTPProxy objects, but it also offers the flexibility to be expanded with multiple chained rules.
When duplicates occur, all the related HTTPProxy objects turn invalid, leading to HTTP 404 errors because the associated configurations are removed from the Envoy proxy.

## How It Works

### Overview:
The `contour-admission-webhook` server relies on an in-memory cache, initialized at startup, to maintain a map of FQDNs to their respective owner references before it begins processing requests. At the forefront of the rule chain is the FQDN validation rule, which leverages this in-memory cache.

### FQDN Validation Flow:
- CREATE/UPDATE Operations:

  When attempting to create or update an HTTPProxy object, the webhook first checks the requested FQDN against the built-in cache. If a match is found, the webhook operation is declined, accompanied by a message detailing the reason.
  If no match is found, the FQDN is added to the cache and the operation gets approved.
  For UPDATE operations specifically, any previous FQDN associated with the object is removed from the cache.

- DELETE Operations:

  Upon executing a DELETE operation, the corresponding FQDN entry is purged from the cache and the operation gets approved.

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
