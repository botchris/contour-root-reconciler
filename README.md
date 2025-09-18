# Contour Root Proxy Reconciler

This package provides a Kubernetes controller that reconciles `HTTPProxy`
resources to ensure that they are correctly configured to use a root proxy.

Each time a child `HTTPProxy` is modified (created, updated, deleted), the
controller checks if it has a `root-proxy` label defined. If so, it retrieves
the corresponding root `HTTPProxy` and appends the child to the root's
`include` section.

This prevents the need to manually update root proxies whenever a new child
proxy is added. Instead, the controller automatically manages the relationship
between root and child proxies based on labels.

## Motivation

In a large Kubernetes cluster with many services, it can be cumbersome to
manually maintain `HTTPProxy` resources, especially when new services are
frequently added or removed. By using a controller to automate the inclusion
of child proxies into root proxies, we can reduce operational overhead and
ensure that the routing configuration remains consistent.

## Usage

1. Deploy the controller in your Kubernetes cluster.
2. Label your child `HTTPProxy` resources with the `root-proxy` label,
   specifying the name of the root `HTTPProxy` they should be included in.
3. The controller will automatically update the root `HTTPProxy` to include
   the child proxies.
4. Monitor the controller logs for any errors or issues.
5. Ensure that the controller has the necessary permissions to read and write
   `HTTPProxy` resources.

### Example

Given a root `HTTPProxy` named `root-proxy`:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: my-root-proxy
spec:
  virtualhost:
    fqdn: example.com
    includes: []
```

And a child `HTTPProxy` named `child-proxy-one` with the appropriate label:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: child-proxy-one
  labels:
    root-proxy: my-root-proxy
spec:
  routes:
    - conditions:
        - prefix: /my-service
      services:
        - name: my-custom-service
          port: 8080
          protocol: h2
```
