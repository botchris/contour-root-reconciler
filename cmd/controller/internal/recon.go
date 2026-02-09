package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	schemav1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler extends the standard reconcile.Reconciler with a SetupWithManager method.
type Reconciler interface {
	reconcile.Reconciler
	SetupWithManager(ctrl.Manager) error
}

type childReconciler struct {
	client client.Client
	logger logr.Logger
}

// NewChildReconciler returns a new reconcile.Reconciler for HTTPProxy children.
func NewChildReconciler(c client.Client, logger logr.Logger) Reconciler {
	return &childReconciler{
		client: c,
		logger: logger,
	}
}

func (r *childReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	child := &schemav1.HTTPProxy{}
	if err := r.client.Get(ctx, req.NamespacedName, child); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger := r.logger.WithValues("child-name", child.Name, "child-namespace", child.Namespace)

	logger.Info("Parsing child HTTPProxy for root annotations")

	rootSelectors, err := r.parseRootRefs(child)
	if err != nil {
		logger.Error(err, "Failed to parse root annotations from child HTTPProxy", "child", child.Name, "namespace", child.Namespace)

		return ctrl.Result{}, nil
	}

	errs := make([]error, 0)

	for i := range rootSelectors {
		reconLogger := logger.WithValues("root-name", rootSelectors[i].Name, "root-namespace", rootSelectors[i].Namespace)

		reconLogger.Info("Reconciling child HTTPProxy")

		rErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rootSchema := &schemav1.HTTPProxy{}

			if gErr := r.client.Get(ctx, rootSelectors[i], rootSchema); gErr != nil {
				return gErr
			}

			return r.reconcileWithRoot(ctx, child, rootSchema)
		})

		if rErr != nil {
			reconLogger.Error(rErr, "Failed to reconcile child with root")

			errs = append(errs, rErr)
		} else {
			reconLogger.Info("Successfully reconciled child with root")
		}
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	return ctrl.Result{}, nil
}

func (r *childReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&schemav1.HTTPProxy{}).Complete(r)
}

func (r *childReconciler) reconcileWithRoot(ctx context.Context, child *schemav1.HTTPProxy, root *schemav1.HTTPProxy) error {
	include := schemav1.Include{Name: child.Name, Namespace: child.Namespace}
	changed := false
	newIncludes := make([]schemav1.Include, 0)

	// Child marked for deletion -> remove from root.
	if child.DeletionTimestamp != nil {
		for i := range root.Spec.Includes {
			if !r.equalsInclude(root.Spec.Includes[i], include) {
				newIncludes = append(newIncludes, root.Spec.Includes[i])
			}
		}

		changed = true
	} else {
		// New child being added -> append to "include" section
		if !r.containsImport(root.Spec.Includes, include) {
			root.Spec.Includes = append(root.Spec.Includes, include)
			changed = true
		}

		// Remove invalid includes already present in the root
		for j := range root.Spec.Includes {
			var c schemav1.HTTPProxy

			if err := r.client.Get(ctx, client.ObjectKey{Namespace: root.Spec.Includes[j].Namespace, Name: root.Spec.Includes[j].Name}, &c); err == nil {
				newIncludes = append(newIncludes, root.Spec.Includes[j])
			} else {
				changed = true
			}
		}
	}

	if changed {
		root.Spec.Includes = r.dedupIncludes(newIncludes)

		return r.client.Update(ctx, root)
	}

	return nil
}

func (r *childReconciler) equalsInclude(a, b schemav1.Include) bool {
	return a.Name == b.Name && a.Namespace == b.Namespace
}

func (r *childReconciler) dedupIncludes(includes []schemav1.Include) []schemav1.Include {
	seen := make(map[string]struct{})
	result := make([]schemav1.Include, 0, len(includes))

	for i := range includes {
		key := fmt.Sprintf("%s/%s", includes[i].Namespace, includes[i].Name)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, includes[i])
		}
	}

	return result
}

func (r *childReconciler) containsImport(stack []schemav1.Include, target schemav1.Include) bool {
	for i := range stack {
		if stack[i].Name == target.Name && stack[i].Namespace == target.Namespace {
			return true
		}
	}

	return false
}

func (r *childReconciler) parseRootRefs(child *schemav1.HTTPProxy) ([]client.ObjectKey, error) {
	// If no "root-proxy" annotation is found, check for the old label-based approach for backward compatibility.
	if _, ok := child.Annotations["root-proxy"]; !ok {
		return r.parseRootLabels(child)
	}

	annotations, hasRoot := child.Annotations["root-proxy"]
	if !hasRoot {
		return nil, nil
	}

	list := strings.Split(annotations, ",")
	out := make([]client.ObjectKey, 0)

	for i := range list {
		rootName := strings.TrimSpace(list[i])
		rootNamespace := child.Namespace

		if strings.Contains(rootName, "[") && strings.Contains(rootName, "]") {
			parts := strings.Split(rootName, "[")
			rootName = strings.TrimSpace(parts[0])
			rootNamespace = strings.TrimSuffix(strings.TrimSpace(parts[1]), "]")
		}

		out = append(out, client.ObjectKey{
			Name:      strings.TrimSpace(rootName),
			Namespace: strings.TrimSpace(rootNamespace),
		})
	}

	return out, nil
}

// parseRootLabels is a helper function to parse root references from labels.
// Deprecated: this function is only used for backward compatibility with the old label-based
// approach and should be removed in future versions.
func (r *childReconciler) parseRootLabels(child *schemav1.HTTPProxy) ([]client.ObjectKey, error) {
	names, hasRoot := child.Labels["root-proxy"]
	if !hasRoot {
		return nil, nil
	}

	namesList := strings.Split(names, ",")
	spacesList := strings.Split(child.Labels["root-proxy-namespace"], ",")

	if len(spacesList) > 0 && len(spacesList) != len(namesList) {
		return nil, fmt.Errorf("invalid root-proxy-namespace label: expected %d namespaces but got %d", len(namesList), len(spacesList))
	}

	out := make([]client.ObjectKey, 0)

	for i := range namesList {
		root := client.ObjectKey{
			Name:      strings.TrimSpace(namesList[i]),
			Namespace: child.Namespace,
		}

		if len(spacesList) > 0 {
			if cleanNamespace := strings.TrimSpace(spacesList[i]); cleanNamespace != "" {
				root.Namespace = cleanNamespace
			}
		}

		out = append(out, root)
	}

	return out, nil
}
