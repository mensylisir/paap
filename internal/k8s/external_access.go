package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func SetNamespaceServiceExternalAccess(ctx context.Context, namespace, serviceType string, enabled bool) (*corev1.Service, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	services := &corev1.ServiceList{}
	if err := cl.List(ctx, services, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	target := externalAccessTargetService(services.Items, serviceType)
	if target == nil {
		return nil, fmt.Errorf("no service in namespace %s can be exposed", namespace)
	}

	next := target.DeepCopy()
	if enabled {
		next.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		next.Spec.Type = corev1.ServiceTypeClusterIP
		for i := range next.Spec.Ports {
			next.Spec.Ports[i].NodePort = 0
		}
	}
	if err := cl.Update(ctx, next); err != nil {
		return nil, fmt.Errorf("update service %s/%s: %w", next.Namespace, next.Name, err)
	}
	return next, nil
}

func findComponentService(ctx context.Context, cl client.Client, namespace, componentIdentifier string) (*corev1.Service, error) {
	services := &corev1.ServiceList{}
	selector := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{"paap.io/component": componentIdentifier},
	}
	if err := cl.List(ctx, services, selector...); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	if len(services.Items) == 0 {
		fallback := []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels{"app": componentIdentifier},
		}
		if err := cl.List(ctx, services, fallback...); err != nil {
			return nil, fmt.Errorf("list services (fallback): %w", err)
		}
	}
	if len(services.Items) == 0 {
		return nil, fmt.Errorf("no service found for component %s in namespace %s", componentIdentifier, namespace)
	}
	return &services.Items[0], nil
}

// SetComponentExternalAccess creates or removes an Ingress for a component's Service.
func SetComponentExternalAccess(ctx context.Context, namespace, componentIdentifier string, enabled bool) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}

	target, err := findComponentService(ctx, cl, namespace, componentIdentifier)
	if err != nil {
		return err
	}
	if len(target.Spec.Ports) == 0 {
		return fmt.Errorf("service %s/%s has no ports", namespace, target.Name)
	}

	ingressName := "comp-" + componentIdentifier
	servicePort := target.Spec.Ports[0]

	if enabled {
		pathType := networkingv1.PathTypePrefix
		ingress := &networkingv1.Ingress{
			ObjectMeta: ingressObjectMeta(namespace, ingressName, componentIdentifier),
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: componentIdentifier + "." + namespace + ".paap.local",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: target.Name,
												Port: networkingv1.ServiceBackendPort{Number: servicePort.Port},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		existing := &networkingv1.Ingress{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ingressName}, existing); err == nil {
			existing.Spec = ingress.Spec
			existing.SetLabels(ingress.GetLabels())
			existing.SetAnnotations(ingress.GetAnnotations())
			return cl.Update(ctx, existing)
		}
		return cl.Create(ctx, ingress)
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: ingressObjectMeta(namespace, ingressName, componentIdentifier),
	}
	if err := cl.Delete(ctx, ingress); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func ingressObjectMeta(namespace, name, componentIdentifier string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			"paap.io/component": componentIdentifier,
			"paap.io/managed":   "true",
		},
		Annotations: map[string]string{
			"paap.io/ingress-purpose": "component-external-access",
		},
	}
}

func SetComponentNodePortAccess(ctx context.Context, namespace, componentIdentifier string, enabled bool) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	target, err := findComponentService(ctx, cl, namespace, componentIdentifier)
	if err != nil {
		return err
	}
	next := target.DeepCopy()
	if enabled {
		next.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		next.Spec.Type = corev1.ServiceTypeClusterIP
		for i := range next.Spec.Ports {
			next.Spec.Ports[i].NodePort = 0
		}
	}
	return cl.Update(ctx, next)
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

func externalAccessTargetService(services []corev1.Service, serviceType string) *corev1.Service {
	bestScore := -1
	var best *corev1.Service
	for i := range services {
		score := externalAccessServiceScore(services[i], serviceType)
		if score < 0 {
			continue
		}
		if score > bestScore || (score == bestScore && best != nil && services[i].Name < best.Name) {
			bestScore = score
			best = &services[i]
		}
	}
	return best
}

func externalAccessServiceScore(svc corev1.Service, serviceType string) int {
	name := strings.ToLower(svc.Name)
	normalizedType := strings.ToLower(strings.TrimSpace(serviceType))
	component := strings.ToLower(strings.TrimSpace(svc.Labels["app.kubernetes.io/component"]))
	if svc.Spec.Type == corev1.ServiceTypeExternalName || svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return -1
	}
	if strings.Contains(name, "headless") || strings.HasSuffix(name, "-hl") || strings.Contains(name, "metrics") || strings.Contains(name, "exporter") {
		return -1
	}
	if len(svc.Spec.Ports) == 0 {
		return -1
	}
	hasExposablePort := false
	for _, port := range svc.Spec.Ports {
		if !skipExternalServicePort(port) {
			hasExposablePort = true
			break
		}
	}
	if !hasExposablePort {
		return -1
	}

	score := 10
	if normalizedType != "" && (strings.Contains(name, normalizedType) || strings.EqualFold(svc.Labels["app.kubernetes.io/name"], normalizedType)) {
		score += 20
	}
	switch normalizedType {
	case "redis":
		if component == "master" || strings.Contains(name, "master") {
			score += 80
		}
		if component == "replica" || strings.Contains(name, "replica") {
			score -= 30
		}
	case "mysql", "postgresql":
		if component == "primary" || strings.Contains(name, "primary") {
			score += 80
		}
		if component == "secondary" || component == "read" || strings.Contains(name, "secondary") || strings.Contains(name, "read") {
			score -= 30
		}
	default:
		if component != "" {
			score += 5
		}
	}
	return score
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
