package internal

import (
	"context"
	"fmt"

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

	logger := r.logger.WithValues(
		"child", req.NamespacedName,
		"resourceVersion", child.ResourceVersion,
		"deletionTimestamp", child.DeletionTimestamp,
	)

	rootName, hasRoot := child.Labels["root-proxy"]
	if !hasRoot {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling child HTTPProxy", "root", rootName)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		root := &schemav1.HTTPProxy{}
		rootKey := client.ObjectKey{Namespace: child.Namespace, Name: rootName}

		if err := r.client.Get(ctx, rootKey, root); err != nil {
			return err
		}

		eErr := r.reconcileChild(ctx, root, child)
		if eErr != nil {
			logger.Error(eErr, "Failed to reconcile child with root", "root", rootKey)
		} else {
			logger.Info("Successfully reconciled child with root", "root", rootKey)
		}

		return eErr
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *childReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&schemav1.HTTPProxy{}).Complete(r)
}

func (r *childReconciler) reconcileChild(ctx context.Context, root *schemav1.HTTPProxy, child *schemav1.HTTPProxy) error {
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
