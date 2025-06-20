package internal

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

type Fetcher struct {
	clientset kubernetes.Interface
}

func NewFetcher(clientset kubernetes.Interface) *Fetcher {
	return &Fetcher{
		clientset: clientset,
	}
}

func (f *Fetcher) FetchNodes(ctx context.Context) error {
	return nil
}

func (f *Fetcher) FetchPendingPods(ctx context.Context, namespace string) error {
	return nil
}
