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

## Features

- Automatically updates root `HTTPProxy` resources to include child proxies
  based on labels.
- Supports specifying the namespace of the root proxy if it differs from
  the child proxy's namespace.
- Handles creation, updates, and deletion of child proxies.
- Ensures that the root proxy always reflects the current set of child proxies.
- Lightweight and easy to deploy in any Kubernetes cluster using Contour.
- Open source and community-driven.

## Installation

You can install the controller using the provided YAML manifest or via Helm.
Whatever method you choose, the controller requires access to the Kubernetes API
and permissions to read and modify `HTTPProxy` resources.

Also, the following flags can be used to customize the controller's behavior:

- `metrics-bind-address` (default `:8080`): The address the metric endpoint binds to.
- `health-probe-bind-address` (default `:8081`): The address the probe endpoint binds to.
- `leader-elect` (default `true`): Enable leader election for controller manager.
  Enabling this will ensure there is only one active controller manager.

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

## Usage

Label your child `HTTPProxy` resources with the `root-proxy` label, specifying
the name of the root proxy.

The reconciler assumes that the root proxy is in the same namespace as the
child proxy. If your root proxy is in a different namespace, you can use
the `root-proxy-namespace` label to specify the namespace where the root
proxy resides.

### Example

Given a root `HTTPProxy` named `my-root-proxy` in the namespace
`my-root-namespace`:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: my-root-proxy
  namespace: my-root-namespace
spec:
  virtualhost:
    fqdn: example.com
    includes: []
```

And a child `HTTPProxy` named `child-proxy-one` located in the namespace
`my-child-namespace`, labeled to indicate it should be included in the root
proxy:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: child-proxy-one
  namespace: my-child-namespace
  labels:
    root-proxy: my-root-proxy # This label indicates the root proxy name
    root-proxy-namespace: my-root-namespace # Optional: specify the root proxy namespace if different from the child proxy's namespace
spec:
  routes:
    - conditions:
        - prefix: /my-service
      services:
        - name: my-backend-service
          port: 8080
          protocol: h2
```

The controller will automatically update `my-root-proxy` to include
`child-proxy-one`, resulting in something like follows:

```yaml
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: my-root-proxy
  namespace: my-root-namespace
spec:
  virtualhost:
    fqdn: example.com
    includes:
      - name: child-proxy-one
        namespace: my-child-namespace
```
