package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExternalEndpoint struct {
	Name string
	Kind string
	URL  string
}

func ListNamespaceExternalEndpoints(ctx context.Context, namespace string) ([]ExternalEndpoint, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}

	nodeHosts := nodeAccessHosts(ctx, cl)

	endpoints := make([]ExternalEndpoint, 0)
	services := &corev1.ServiceList{}
	if err := cl.List(ctx, services, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	for _, svc := range services.Items {
		endpoints = append(endpoints, serviceExternalEndpoints(svc, nodeHosts)...)
	}

	ingresses := &networkingv1.IngressList{}
	if err := cl.List(ctx, ingresses, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list ingresses: %w", err)
	}
	for _, ingress := range ingresses.Items {
		endpoints = append(endpoints, ingressExternalEndpoints(ingress)...)
	}
	endpoints = append(endpoints, gatewayExternalEndpoints(ctx, cl, namespace)...)

	sort.Slice(endpoints, func(i, j int) bool {
		if endpointPriority(endpoints[i].Kind) != endpointPriority(endpoints[j].Kind) {
			return endpointPriority(endpoints[i].Kind) < endpointPriority(endpoints[j].Kind)
		}
		if endpoints[i].Name != endpoints[j].Name {
			return endpoints[i].Name < endpoints[j].Name
		}
		return endpoints[i].URL < endpoints[j].URL
	})
	return endpoints, nil
}

func endpointPriority(kind string) int {
	switch kind {
	case "Gateway":
		return 0
	case "Ingress":
		return 1
	case "LoadBalancer":
		return 2
	case "NodePort":
		return 3
	default:
		return 9
	}
}

func nodeAccessHosts(ctx context.Context, cl client.Client) []string {
	nodes := &corev1.NodeList{}
	if err := cl.List(ctx, nodes); err != nil {
		return nil
	}
	hosts := make([]string, 0, len(nodes.Items))
	seen := map[string]bool{}
	for _, node := range nodes.Items {
		host := nodeAccessHost(node)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

func nodeAccessHost(node corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeExternalIP && strings.TrimSpace(addr.Address) != "" {
			return strings.TrimSpace(addr.Address)
		}
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP && strings.TrimSpace(addr.Address) != "" {
			return strings.TrimSpace(addr.Address)
		}
	}
	return ""
}

func serviceExternalEndpoints(svc corev1.Service, nodeHosts []string) []ExternalEndpoint {
	endpoints := make([]ExternalEndpoint, 0)
	switch svc.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		for _, ing := range svc.Status.LoadBalancer.Ingress {
			host := strings.TrimSpace(ing.Hostname)
			if host == "" {
				host = strings.TrimSpace(ing.IP)
			}
			if host == "" {
				continue
			}
			for _, port := range svc.Spec.Ports {
				if skipExternalServicePort(port) {
					continue
				}
				endpoints = append(endpoints, ExternalEndpoint{
					Name: svc.Name,
					Kind: "LoadBalancer",
					URL:  servicePortURL(host, port, int(port.Port)),
				})
			}
		}
	case corev1.ServiceTypeNodePort:
		for _, host := range nodeHosts {
			for _, port := range svc.Spec.Ports {
				if port.NodePort == 0 || skipExternalServicePort(port) {
					continue
				}
				endpoints = append(endpoints, ExternalEndpoint{
					Name: svc.Name,
					Kind: "NodePort",
					URL:  servicePortURL(host, port, int(port.NodePort)),
				})
			}
		}
	}
	return endpoints
}

func skipExternalServicePort(port corev1.ServicePort) bool {
	name := strings.ToLower(strings.TrimSpace(port.Name))
	if name == "ssh" || name == "git-ssh" || strings.Contains(name, "ssh") {
		return true
	}
	return port.Port == 22
}

func servicePortURL(host string, svcPort corev1.ServicePort, exposedPort int) string {
	scheme := servicePortScheme(svcPort)
	if exposedPort == 80 && scheme == "http" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	if exposedPort == 443 && scheme == "https" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, exposedPort)
}

func servicePortScheme(port corev1.ServicePort) string {
	scheme := "http"
	name := strings.ToLower(strings.TrimSpace(port.Name))
	appProtocol := ""
	if port.AppProtocol != nil {
		appProtocol = strings.ToLower(strings.TrimSpace(*port.AppProtocol))
	}
	if port.Port == 443 || name == "https" || strings.Contains(name, "https") || appProtocol == "https" {
		scheme = "https"
	}
	return scheme
}

func ingressExternalEndpoints(ingress networkingv1.Ingress) []ExternalEndpoint {
	endpoints := make([]ExternalEndpoint, 0)
	for _, rule := range ingress.Spec.Rules {
		host := strings.TrimSpace(rule.Host)
		if host == "" {
			continue
		}
		scheme := "http"
		if ingressHostUsesTLS(ingress, host) {
			scheme = "https"
		}
		if rule.HTTP == nil || len(rule.HTTP.Paths) == 0 {
			endpoints = append(endpoints, ExternalEndpoint{Name: ingress.Name, Kind: "Ingress", URL: fmt.Sprintf("%s://%s", scheme, host)})
			continue
		}
		for _, path := range rule.HTTP.Paths {
			urlPath := path.Path
			if urlPath == "" {
				urlPath = "/"
			}
			endpoints = append(endpoints, ExternalEndpoint{Name: ingress.Name, Kind: "Ingress", URL: fmt.Sprintf("%s://%s%s", scheme, host, urlPath)})
		}
	}
	return endpoints
}

func ingressHostUsesTLS(ingress networkingv1.Ingress, host string) bool {
	for _, tls := range ingress.Spec.TLS {
		for _, tlsHost := range tls.Hosts {
			if tlsHost == host {
				return true
			}
		}
	}
	return false
}

func gatewayExternalEndpoints(ctx context.Context, cl client.Client, namespace string) []ExternalEndpoint {
	gateways := &unstructured.UnstructuredList{}
	gateways.SetGroupVersionKind(schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "GatewayList"})
	if err := cl.List(ctx, gateways, client.InNamespace(namespace)); err != nil {
		return nil
	}
	routes := &unstructured.UnstructuredList{}
	routes.SetGroupVersionKind(schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRouteList"})
	if err := cl.List(ctx, routes, client.InNamespace(namespace)); err != nil {
		return nil
	}

	listeners := gatewayListeners(gateways.Items)
	endpoints := make([]ExternalEndpoint, 0)
	for _, route := range routes.Items {
		routeHosts := stringListAt(route.Object, "spec", "hostnames")
		for _, parent := range mapListAt(route.Object, "spec", "parentRefs") {
			gatewayName := stringAt(parent, "name")
			if gatewayName == "" {
				continue
			}
			sectionName := stringAt(parent, "sectionName")
			for _, host := range routeHostsForParent(routeHosts, listeners[gatewayListenerKey(gatewayName, sectionName)]) {
				for _, path := range httpRoutePaths(route.Object) {
					scheme := listeners[gatewayListenerKey(gatewayName, sectionName)].Scheme
					if scheme == "" {
						scheme = "http"
					}
					endpoints = append(endpoints, ExternalEndpoint{
						Name: route.GetName(),
						Kind: "Gateway",
						URL:  fmt.Sprintf("%s://%s%s", scheme, host, path),
					})
				}
			}
		}
	}
	return endpoints
}

type gatewayListener struct {
	Hostname string
	Scheme   string
}

func gatewayListeners(gateways []unstructured.Unstructured) map[string]gatewayListener {
	result := map[string]gatewayListener{}
	for _, gateway := range gateways {
		for _, listener := range mapListAt(gateway.Object, "spec", "listeners") {
			name := stringAt(listener, "name")
			if name == "" {
				continue
			}
			protocol := strings.ToLower(stringAt(listener, "protocol"))
			scheme := "http"
			if protocol == "https" || protocol == "tls" {
				scheme = "https"
			}
			result[gatewayListenerKey(gateway.GetName(), name)] = gatewayListener{
				Hostname: stringAt(listener, "hostname"),
				Scheme:   scheme,
			}
		}
	}
	return result
}

func gatewayListenerKey(gatewayName, sectionName string) string {
	return strings.TrimSpace(gatewayName) + "/" + strings.TrimSpace(sectionName)
}

func routeHostsForParent(routeHosts []string, listener gatewayListener) []string {
	if len(routeHosts) > 0 {
		return routeHosts
	}
	if strings.TrimSpace(listener.Hostname) != "" {
		return []string{strings.TrimSpace(listener.Hostname)}
	}
	return nil
}

func httpRoutePaths(route map[string]interface{}) []string {
	seen := map[string]bool{}
	paths := make([]string, 0)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, rule := range mapListAt(route, "spec", "rules") {
		matches := mapListAt(rule, "matches")
		if len(matches) == 0 {
			add("/")
			continue
		}
		for _, match := range matches {
			path, _, _ := unstructured.NestedString(match, "path", "value")
			add(path)
		}
	}
	if len(paths) == 0 {
		add("/")
	}
	sort.Strings(paths)
	return paths
}

func mapListAt(obj map[string]interface{}, fields ...string) []map[string]interface{} {
	items, _, _ := unstructured.NestedSlice(obj, fields...)
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if typed, ok := item.(map[string]interface{}); ok {
			result = append(result, typed)
		}
	}
	return result
}

func stringListAt(obj map[string]interface{}, fields ...string) []string {
	items, _, _ := unstructured.NestedStringSlice(obj, fields...)
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func stringAt(obj map[string]interface{}, fields ...string) string {
	value, _, _ := unstructured.NestedString(obj, fields...)
	return strings.TrimSpace(value)
}
