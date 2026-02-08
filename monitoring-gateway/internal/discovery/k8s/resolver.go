package k8s

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/dreschagin/monitoring-dashboard/monitoring-gateway/internal/discovery"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Resolver discovers upstream services from Kubernetes API.
type Resolver struct {
	clientset        kubernetes.Interface
	namespace        string
	apiSelector      string
	analyzerSelector string
	analyzerRequired bool
}

func NewInClusterResolver(namespace, apiSelector, analyzerSelector string, analyzerRequired bool) (*Resolver, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("build in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client: %w", err)
	}

	return &Resolver{
		clientset:        clientset,
		namespace:        namespace,
		apiSelector:      apiSelector,
		analyzerSelector: analyzerSelector,
		analyzerRequired: analyzerRequired,
	}, nil
}

func NewResolver(clientset kubernetes.Interface, namespace, apiSelector, analyzerSelector string, analyzerRequired bool) *Resolver {
	return &Resolver{
		clientset:        clientset,
		namespace:        namespace,
		apiSelector:      apiSelector,
		analyzerSelector: analyzerSelector,
		analyzerRequired: analyzerRequired,
	}
}

func (r *Resolver) Resolve(ctx context.Context) (discovery.Snapshot, error) {
	apiURL, err := r.resolveServiceURL(ctx, r.apiSelector, 80)
	if err != nil {
		return discovery.Snapshot{}, fmt.Errorf("resolve api service: %w", err)
	}

	analyzerURL, err := r.resolveServiceURL(ctx, r.analyzerSelector, 8081)
	if err != nil {
		if r.analyzerRequired {
			return discovery.Snapshot{}, fmt.Errorf("resolve analyzer service: %w", err)
		}
	}

	return discovery.Snapshot{
		APIURL:      apiURL,
		AnalyzerURL: analyzerURL,
	}, nil
}

func (r *Resolver) resolveServiceURL(ctx context.Context, selector string, fallbackPort int32) (*url.URL, error) {
	services, err := r.clientset.CoreV1().Services(r.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list services by selector %q: %w", selector, err)
	}

	if len(services.Items) == 0 {
		return nil, fmt.Errorf("no services found for selector %q", selector)
	}

	sort.Slice(services.Items, func(i, j int) bool {
		return services.Items[i].Name < services.Items[j].Name
	})

	svc := services.Items[0]
	port := fallbackPort
	if len(svc.Spec.Ports) > 0 {
		port = svc.Spec.Ports[0].Port
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Name == "http" {
				port = svcPort.Port
				break
			}
		}
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, r.namespace)
	upstreamURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return nil, fmt.Errorf("parse upstream URL for service %q: %w", svc.Name, err)
	}

	return upstreamURL, nil
}
