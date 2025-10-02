package internal

import (
	"context"
	"testing"
	"time"

	schemav1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcile_AddsChildToRoot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	scheme := runtime.NewScheme()
	require.NoError(t, schemav1.AddToScheme(scheme))

	root := &schemav1.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "root",
			Namespace: "default",
		},
		Spec: schemav1.HTTPProxySpec{
			Includes: []schemav1.Include{},
		},
	}

	child := &schemav1.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "child-1",
			Namespace: "test",
			Labels: map[string]string{
				"root-proxy":           "root",
				"root-proxy-namespace": "default",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(root, child).
		Build()

	fakeLogger := ctrl.Log.WithName("test")
	reconciler := NewChildReconciler(fakeClient, fakeLogger).(*childReconciler)

	// Run reconcile
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clientObjectKey(child)})
	assert.NoError(t, err)

	// Fetch updated root
	updatedRoot := &schemav1.HTTPProxy{}
	err = fakeClient.Get(ctx, clientObjectKey(root), updatedRoot)
	assert.NoError(t, err)

	assert.Len(t, updatedRoot.Spec.Includes, 1)
	assert.Equal(t, "child-1", updatedRoot.Spec.Includes[0].Name)
}

func TestReconcile_RemovesDeletedChildFromRoot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	scheme := runtime.NewScheme()
	require.NoError(t, schemav1.AddToScheme(scheme))

	root := &schemav1.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "root",
			Namespace: "default",
		},
		Spec: schemav1.HTTPProxySpec{
			Includes: []schemav1.Include{
				{Name: "child-1", Namespace: "default"},
			},
		},
	}

	now := metav1.NewTime(time.Now())
	child := &schemav1.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "child-1",
			Namespace:         "default",
			Labels:            map[string]string{"root-proxy": "root"},
			DeletionTimestamp: &now,
			Finalizers:        []string{"test.finalizer"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(root, child).
		Build()

	fakeLogger := ctrl.Log.WithName("test")
	reconciler := NewChildReconciler(fakeClient, fakeLogger).(*childReconciler)
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: clientObjectKey(child)})
	assert.NoError(t, err)

	updatedRoot := &schemav1.HTTPProxy{}
	assert.NoError(t, fakeClient.Get(ctx, clientObjectKey(root), updatedRoot))
	assert.Len(t, updatedRoot.Spec.Includes, 0)
}

// helper.
func clientObjectKey(obj *schemav1.HTTPProxy) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Name,
	}
}
