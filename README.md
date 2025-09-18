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

In a large Kubernetes cluster with many services under the same domain
(e.g., `example.com`), it is common to use a root `HTTPProxy` to route traffic
to various child proxies based on path prefixes. However, manually updating the
root proxy each time a new child proxy (service) is created/removed can
be error-prone and tedious.

By using a controller to automate the inclusion of child proxies into root
proxies, we can reduce operational overhead and ensure that the routing
configuration remains consistent.

### Example

Given a root `HTTPProxy` named `my-root-proxy`:

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
    root-proxy: my-root-proxy # This label indicates the root proxy
spec:
  routes:
    - conditions:
        - prefix: /my-service
      services:
        - name: my-custom-service
          port: 8080
          protocol: h2
```

The controller will automatically update `my-root-proxy` to include
`child-proxy-one` as follow:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: my-root-proxy
spec:
  virtualhost:
    fqdn: example.com
    includes:
      - name: child-proxy-one
        namespace: <namespace-of-child-proxy>
```

## Installation

You can install the controller using the provided YAML manifest or via Helm.

### Helm Chart (Recommended)

You can also install the controller using Helm. First, add the Helm repository
and then install the chart:

```shell
helm repo add contour-root-reconciler https://botchris.github.io/contour-root-reconciler
helm repo update
helm install my-reconciler contour-root-reconciler/contour-root-reconciler
```

### Docker Image

You can use the Docker image `botchrishub/contour-root-reconciler:latest`,
and manually deploy the controller in your Kubernetes cluster using a
Deployment manifest. Use the example file located at `example/deployment.yaml`
as a starting point.
