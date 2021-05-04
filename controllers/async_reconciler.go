package controllers

import (
	"context"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

type asyncReconciler struct {
	results chan error
}

func NewAsyncReconciler() asyncReconciler {
	return asyncReconciler{}
}

func (ar *asyncReconciler) submit(ctx context.Context, task azure.Reconciler) {
	go func() {
		ar.results <- task.Reconcile(ctx)

	}()
}

func (ar *asyncReconciler) wait() error {
	var errs []error
	for r := range ar.results {
		if r != nil {
			errs = append(errs, r)
		}
	}

	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}

	return nil
}
