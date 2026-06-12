package k8s

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ArgoCDResource struct {
	Kind       string
	Name       string
	Namespace  string
	Status     string
	Health     string
	Group      string
	Version    string
	UID        string
	ParentRefs []ArgoCDResourceRef
	Orphaned   bool
	Children   []ArgoCDResource
}

type ArgoCDResourceRef struct {
	Kind      string
	Name      string
	Namespace string
	Group     string
	UID       string
}

const argoCDResourcesFinalizer = "resources-finalizer.argocd.argoproj.io"

type ArgoCDApplication struct {
	Name         string
	SyncStatus   string
	HealthStatus string
	RepoURL      string
	Path         string
	Namespace    string
	Server       string
	Revision     string
	Resources    []ArgoCDResource
}

type ArgoCDApplicationSpec struct {
	Name                 string
	Namespace            string
	Project              string
	RepoURL              string
	Path                 string
	TargetRevision       string
	DestinationServer    string
	DestinationNamespace string
	Automated            bool
}

type ArgoCDApplicationSetSpec struct {
	Name                 string
	Namespace            string
	Project              string
	RepoURL              string
	Path                 string
	TargetRevision       string
	DestinationServer    string
	DestinationNamespace string
}

type ArgoCDClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
	token      string
}

func NewArgoCDClient(namespace string) *ArgoCDClient {
	fallback := fmt.Sprintf("http://%s-argocd-server.%s.svc.cluster.local", namespace, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "argocd-server", fallback)
	user, pass := discoverArgoCDCreds(ctx, namespace)
	return &ArgoCDClient{
		BaseURL:  baseURL,
		Username: user,
		Password: pass,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func discoverArgoCDCreds(ctx context.Context, namespace string) (string, string) {
	cl, err := requireClient()
	if err != nil {
		return "admin", ""
	}
	secret := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "argocd-initial-admin-secret"}, secret); err == nil {
		if pass := strings.TrimSpace(string(secret.Data["password"])); pass != "" {
			return "admin", pass
		}
	}
	var list corev1.SecretList
	if err := cl.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return "admin", ""
	}
	for _, sec := range list.Items {
		if strings.Contains(sec.Name, "argocd") && strings.Contains(sec.Name, "initial") {
			if pass := strings.TrimSpace(string(sec.Data["password"])); pass != "" {
				return "admin", pass
			}
		}
	}
	return "admin", ""
}

func EnsureArgoCDEnvironmentProject(ctx context.Context, namespace, name, repoURL string, destinationNamespaces []string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if namespace == "" || name == "" || repoURL == "" {
		return fmt.Errorf("namespace, project name and repoURL are required")
	}

	destinations := make([]interface{}, 0, len(destinationNamespaces))
	seen := map[string]bool{}
	for _, destinationNamespace := range destinationNamespaces {
		destinationNamespace = strings.TrimSpace(destinationNamespace)
		if destinationNamespace == "" || seen[destinationNamespace] {
			continue
		}
		seen[destinationNamespace] = true
		destinations = append(destinations, map[string]interface{}{
			"server":    "https://kubernetes.default.svc",
			"namespace": destinationNamespace,
		})
	}
	if len(destinations) == 0 {
		return fmt.Errorf("argocd project destination namespaces are required")
	}

	namespaceWhitelist := []interface{}{
		map[string]interface{}{"group": "", "kind": "ConfigMap"},
		map[string]interface{}{"group": "", "kind": "Endpoints"},
		map[string]interface{}{"group": "", "kind": "Event"},
		map[string]interface{}{"group": "", "kind": "PersistentVolumeClaim"},
		map[string]interface{}{"group": "", "kind": "Pod"},
		map[string]interface{}{"group": "", "kind": "Secret"},
		map[string]interface{}{"group": "", "kind": "Service"},
		map[string]interface{}{"group": "", "kind": "ServiceAccount"},
		map[string]interface{}{"group": "apps", "kind": "ControllerRevision"},
		map[string]interface{}{"group": "apps", "kind": "Deployment"},
		map[string]interface{}{"group": "apps", "kind": "ReplicaSet"},
		map[string]interface{}{"group": "apps", "kind": "StatefulSet"},
		map[string]interface{}{"group": "autoscaling", "kind": "HorizontalPodAutoscaler"},
		map[string]interface{}{"group": "batch", "kind": "CronJob"},
		map[string]interface{}{"group": "batch", "kind": "Job"},
		map[string]interface{}{"group": "discovery.k8s.io", "kind": "EndpointSlice"},
		map[string]interface{}{"group": "networking.k8s.io", "kind": "Ingress"},
	}
	labels := map[string]string{"paap.io/managed-by": "paap-server"}

	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	err = cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, project)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get argocd project: %w", err)
	}
	sourceRepos := []interface{}{repoURL}
	if err == nil {
		existingRepos, ok, _ := unstructured.NestedSlice(project.Object, "spec", "sourceRepos")
		if ok {
			seenRepos := map[string]bool{repoURL: true}
			for _, item := range existingRepos {
				existingRepo := strings.TrimSpace(fmt.Sprint(item))
				if existingRepo == "" || existingRepo == "*" || seenRepos[existingRepo] {
					continue
				}
				seenRepos[existingRepo] = true
				sourceRepos = append(sourceRepos, existingRepo)
			}
		}
	}
	spec := map[string]interface{}{
		"description":                "PAAP environment project",
		"sourceRepos":                sourceRepos,
		"destinations":               destinations,
		"clusterResourceWhitelist":   []interface{}{},
		"namespaceResourceWhitelist": namespaceWhitelist,
	}
	if apierrors.IsNotFound(err) {
		project = &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
		project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
		project.SetNamespace(namespace)
		project.SetName(name)
		project.SetLabels(labels)
		return cl.Create(ctx, project)
	}
	project.SetLabels(mergeLabels(project.GetLabels(), labels))
	if err := unstructured.SetNestedField(project.Object, spec, "spec"); err != nil {
		return err
	}
	project.SetManagedFields(nil)
	return cl.Update(ctx, project)
}

func EnsureArgoCDLocalClusterSecret(ctx context.Context, namespace string, destinationNamespaces []string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if namespace == "" {
		return fmt.Errorf("argocd namespace is required")
	}
	namespaces := make([]string, 0, len(destinationNamespaces))
	seen := map[string]bool{}
	for _, item := range destinationNamespaces {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		namespaces = append(namespaces, item)
	}
	sort.Strings(namespaces)
	if len(namespaces) == 0 {
		return fmt.Errorf("argocd local cluster namespaces are required")
	}

	config := map[string]interface{}{
		"inCluster": true,
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	data := map[string][]byte{
		"name":             []byte("in-cluster"),
		"server":           []byte("https://kubernetes.default.svc"),
		"namespaces":       []byte(strings.Join(namespaces, ",")),
		"clusterResources": []byte("false"),
		"config":           configJSON,
	}
	labels := map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		"paap.io/managed-by":             "paap-server",
	}

	secret := &corev1.Secret{}
	err = cl.Get(ctx, types.NamespacedName{Name: "paap-local-cluster", Namespace: namespace}, secret)
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "paap-local-cluster",
				Namespace: namespace,
				Labels:    labels,
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		return cl.Create(ctx, secret)
	}
	if err != nil {
		return fmt.Errorf("get argocd local cluster secret: %w", err)
	}
	secret.SetLabels(mergeLabels(secret.GetLabels(), labels))
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	for key, value := range data {
		secret.Data[key] = value
	}
	secret.SetManagedFields(nil)
	return cl.Update(ctx, secret)
}

func EnsureArgoCDDefaultProjectDenied(ctx context.Context, namespace string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	spec := map[string]interface{}{
		"description":                "PAAP disables the default project; use the environment project instead.",
		"sourceRepos":                []interface{}{},
		"destinations":               []interface{}{},
		"clusterResourceWhitelist":   []interface{}{},
		"namespaceResourceWhitelist": []interface{}{},
	}
	labels := map[string]string{"paap.io/managed-by": "paap-server"}

	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	err = cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "default"}, project)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get default argocd project: %w", err)
	}
	if apierrors.IsNotFound(err) {
		project = &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
		project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
		project.SetNamespace(namespace)
		project.SetName("default")
		project.SetLabels(labels)
		return cl.Create(ctx, project)
	}
	project.SetLabels(mergeLabels(project.GetLabels(), labels))
	if err := unstructured.SetNestedField(project.Object, spec, "spec"); err != nil {
		return err
	}
	project.SetManagedFields(nil)
	return cl.Update(ctx, project)
}

func ListArgoCDApplications(ctx context.Context, namespace string) ([]ArgoCDApplication, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationList"})
	if err := cl.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list argocd applications: %w", err)
	}
	apps := make([]ArgoCDApplication, 0, len(list.Items))
	for _, item := range list.Items {
		repoURL, _, _ := unstructured.NestedString(item.Object, "spec", "source", "repoURL")
		path, _, _ := unstructured.NestedString(item.Object, "spec", "source", "path")
		syncStatus, _, _ := unstructured.NestedString(item.Object, "status", "sync", "status")
		healthStatus, _, _ := unstructured.NestedString(item.Object, "status", "health", "status")
		destNS, _, _ := unstructured.NestedString(item.Object, "spec", "destination", "namespace")
		destServer, _, _ := unstructured.NestedString(item.Object, "spec", "destination", "server")
		revision, _, _ := unstructured.NestedString(item.Object, "status", "sync", "revision")

		resources := make([]ArgoCDResource, 0)
		if resList, ok, _ := unstructured.NestedSlice(item.Object, "status", "resources"); ok && len(resList) > 0 {
			for _, r := range resList {
				m, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				rk, _, _ := unstructured.NestedString(m, "kind")
				rn, _, _ := unstructured.NestedString(m, "name")
				rns, _, _ := unstructured.NestedString(m, "namespace")
				rs, _, _ := unstructured.NestedString(m, "status")
				rh, _, _ := unstructured.NestedString(m, "health", "status")
				if rk != "" && rn != "" {
					resources = append(resources, ArgoCDResource{
						Kind:      rk,
						Name:      rn,
						Namespace: rns,
						Status:    rs,
						Health:    rh,
					})
				}
			}
		}

		apps = append(apps, ArgoCDApplication{
			Name:         item.GetName(),
			SyncStatus:   valueOrUnknown(syncStatus),
			HealthStatus: valueOrUnknown(healthStatus),
			RepoURL:      repoURL,
			Path:         path,
			Namespace:    destNS,
			Server:       destServer,
			Revision:     revision,
			Resources:    resources,
		})
	}
	return apps, nil
}

func (a *ArgoCDClient) ResourceTree(ctx context.Context, application string) ([]ArgoCDResource, error) {
	application = strings.TrimSpace(application)
	if application == "" {
		return nil, fmt.Errorf("application is required")
	}
	if a.token == "" {
		if err := a.login(ctx); err != nil {
			return nil, err
		}
	}
	endpoint := strings.TrimRight(a.BaseURL, "/") + "/api/v1/applications/" + url.PathEscape(application) + "/resource-tree"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	res, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("argocd resource tree request failed: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("argocd resource tree returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Nodes         []argoCDResourceTreeNode `json:"nodes"`
		OrphanedNodes []argoCDResourceTreeNode `json:"orphanedNodes"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode argocd resource tree: %w", err)
	}
	return argoCDTreeNodesToResources(payload.Nodes, payload.OrphanedNodes), nil
}

func (a *ArgoCDClient) login(ctx context.Context) error {
	if strings.TrimSpace(a.Password) == "" {
		return fmt.Errorf("argocd admin password secret not found")
	}
	body := strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, a.Username, a.Password))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.BaseURL, "/")+"/api/v1/session", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("argocd login request failed: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("argocd login returned %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode argocd login response: %w", err)
	}
	if strings.TrimSpace(payload.Token) == "" {
		return fmt.Errorf("argocd login response did not include token")
	}
	a.token = payload.Token
	return nil
}

type argoCDResourceTreeNode struct {
	Kind            string                   `json:"kind"`
	Name            string                   `json:"name"`
	Namespace       string                   `json:"namespace"`
	Group           string                   `json:"group"`
	Version         string                   `json:"version"`
	UID             string                   `json:"uid"`
	ParentRefs      []argoCDResourceRef      `json:"parentRefs"`
	Health          map[string]interface{}   `json:"health"`
	Info            []map[string]interface{} `json:"info"`
	ResourceVersion string                   `json:"resourceVersion"`
}

type argoCDResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Group     string `json:"group"`
	UID       string `json:"uid"`
}

func argoCDTreeNodesToResources(nodes []argoCDResourceTreeNode, orphaned []argoCDResourceTreeNode) []ArgoCDResource {
	all := append([]argoCDResourceTreeNode{}, nodes...)
	all = append(all, orphaned...)
	orphanedKeys := map[string]bool{}
	for _, node := range orphaned {
		if key := argoCDNodeKey(node); key != "" {
			orphanedKeys[key] = true
		}
	}
	byKey := map[string]*argoCDResourceTreeItem{}
	order := make([]string, 0, len(all))
	for _, node := range all {
		key := argoCDNodeKey(node)
		if key == "" {
			continue
		}
		resource := argoCDNodeResource(node)
		resource.Orphaned = orphanedKeys[key]
		item := &argoCDResourceTreeItem{resource: resource}
		byKey[key] = item
		for _, alias := range argoCDNodeAliases(node) {
			byKey[alias] = item
		}
		order = append(order, key)
	}
	childKeys := map[string]struct{}{}
	for _, node := range all {
		childKey := argoCDNodeKey(node)
		child := byKey[childKey]
		if child == nil || len(node.ParentRefs) == 0 {
			continue
		}
		for _, ref := range node.ParentRefs {
			parent := byKey[argoCDRefKey(ref)]
			if parent == nil {
				continue
			}
			parent.children = append(parent.children, child)
			childKeys[childKey] = struct{}{}
			break
		}
	}
	roots := make([]ArgoCDResource, 0)
	for _, key := range order {
		if _, isChild := childKeys[key]; isChild {
			continue
		}
		if resource := byKey[key]; resource != nil {
			roots = append(roots, resource.clone())
		}
	}
	return roots
}

type argoCDResourceTreeItem struct {
	resource ArgoCDResource
	children []*argoCDResourceTreeItem
}

func (i *argoCDResourceTreeItem) clone() ArgoCDResource {
	out := i.resource
	out.Children = make([]ArgoCDResource, 0, len(i.children))
	for _, child := range i.children {
		out.Children = append(out.Children, child.clone())
	}
	return out
}

func argoCDNodeResource(node argoCDResourceTreeNode) ArgoCDResource {
	health := ""
	if value, ok := node.Health["status"]; ok {
		health = strings.TrimSpace(fmt.Sprint(value))
	}
	parentRefs := make([]ArgoCDResourceRef, 0, len(node.ParentRefs))
	for _, ref := range node.ParentRefs {
		parentRefs = append(parentRefs, ArgoCDResourceRef{
			Kind:      ref.Kind,
			Name:      ref.Name,
			Namespace: ref.Namespace,
			Group:     ref.Group,
			UID:       ref.UID,
		})
	}
	return ArgoCDResource{
		Kind:       node.Kind,
		Name:       node.Name,
		Namespace:  node.Namespace,
		Status:     valueOrUnknown(health),
		Health:     health,
		Group:      node.Group,
		Version:    node.Version,
		UID:        node.UID,
		ParentRefs: parentRefs,
	}
}

func argoCDNodeKey(node argoCDResourceTreeNode) string {
	if node.UID != "" {
		return "uid:" + node.UID
	}
	return strings.Join([]string{node.Group, node.Kind, node.Namespace, node.Name}, "/")
}

func argoCDNodeAliases(node argoCDResourceTreeNode) []string {
	aliases := []string{
		strings.Join([]string{node.Group, node.Kind, node.Namespace, node.Name}, "/"),
	}
	if node.Group == "" {
		aliases = append(aliases, strings.Join([]string{"", node.Kind, node.Namespace, node.Name}, "/"))
	}
	return aliases
}

func argoCDRefKey(ref argoCDResourceRef) string {
	if ref.UID != "" {
		return "uid:" + ref.UID
	}
	return strings.Join([]string{ref.Group, ref.Kind, ref.Namespace, ref.Name}, "/")
}

func SyncArgoCDApplication(ctx context.Context, namespace, name string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, app); err != nil {
		return fmt.Errorf("get argocd application: %w", err)
	}
	operation := map[string]interface{}{
		"sync": map[string]interface{}{
			"revision": "HEAD",
			"prune":    true,
		},
	}
	if err := unstructured.SetNestedField(app.Object, operation, "operation"); err != nil {
		return err
	}
	app.SetManagedFields(nil)
	return cl.Update(ctx, app)
}

func ApplyArgoCDApplication(ctx context.Context, spec ArgoCDApplicationSpec) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	spec.Project = strings.TrimSpace(spec.Project)
	spec.DestinationNamespace = strings.TrimSpace(spec.DestinationNamespace)
	if spec.Project == "" {
		return fmt.Errorf("project is required")
	}
	if spec.Name == "" || spec.Namespace == "" || spec.RepoURL == "" || spec.Path == "" || spec.DestinationNamespace == "" {
		return fmt.Errorf("name, namespace, project, repoURL, path and destination namespace are required")
	}
	if err := validateArgoCDScopedDestination(spec.Project, spec.DestinationNamespace); err != nil {
		return err
	}
	if spec.TargetRevision == "" {
		spec.TargetRevision = "HEAD"
	}
	if spec.DestinationServer == "" {
		spec.DestinationServer = "https://kubernetes.default.svc"
	}

	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	err = cl.Get(ctx, types.NamespacedName{Namespace: spec.Namespace, Name: spec.Name}, app)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get argocd application: %w", err)
	}
	if apierrors.IsNotFound(err) {
		app = &unstructured.Unstructured{}
		app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
		app.SetName(spec.Name)
		app.SetNamespace(spec.Namespace)
	}

	source := map[string]interface{}{
		"repoURL":        spec.RepoURL,
		"path":           spec.Path,
		"targetRevision": spec.TargetRevision,
	}
	destination := map[string]interface{}{
		"server":    spec.DestinationServer,
		"namespace": spec.DestinationNamespace,
	}
	syncPolicy := map[string]interface{}{}
	if spec.Automated {
		syncPolicy["automated"] = map[string]interface{}{
			"prune":    true,
			"selfHeal": true,
		}
	}
	app.Object["spec"] = map[string]interface{}{
		"project":     spec.Project,
		"source":      source,
		"destination": destination,
		"syncPolicy":  syncPolicy,
	}
	ensureObjectFinalizer(app, argoCDResourcesFinalizer)
	app.SetManagedFields(nil)
	if err != nil {
		return cl.Create(ctx, app)
	}
	return cl.Update(ctx, app)
}

func ApplyArgoCDApplicationSet(ctx context.Context, spec ArgoCDApplicationSetSpec) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	spec.Project = strings.TrimSpace(spec.Project)
	spec.DestinationNamespace = strings.TrimSpace(spec.DestinationNamespace)
	if spec.Project == "" {
		return fmt.Errorf("project is required")
	}
	if spec.Name == "" || spec.Namespace == "" || spec.RepoURL == "" || spec.Path == "" || spec.DestinationNamespace == "" {
		return fmt.Errorf("name, namespace, project, repoURL, path and destination namespace are required")
	}
	if err := validateArgoCDScopedDestination(spec.Project, spec.DestinationNamespace); err != nil {
		return err
	}
	if spec.TargetRevision == "" {
		spec.TargetRevision = "HEAD"
	}
	if spec.DestinationServer == "" {
		spec.DestinationServer = "https://kubernetes.default.svc"
	}

	appSet := &unstructured.Unstructured{}
	appSet.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationSet"})
	err = cl.Get(ctx, types.NamespacedName{Namespace: spec.Namespace, Name: spec.Name}, appSet)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get argocd applicationset: %w", err)
	}
	if apierrors.IsNotFound(err) {
		appSet = &unstructured.Unstructured{}
		appSet.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationSet"})
		appSet.SetName(spec.Name)
		appSet.SetNamespace(spec.Namespace)
	}

	appSet.Object["spec"] = map[string]interface{}{
		"generators": []interface{}{
			map[string]interface{}{
				"git": map[string]interface{}{
					"repoURL":  spec.RepoURL,
					"revision": spec.TargetRevision,
					"directories": []interface{}{
						map[string]interface{}{"path": spec.Path},
					},
				},
			},
		},
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "{{path.basename}}",
			},
			"spec": map[string]interface{}{
				"project": spec.Project,
				"source": map[string]interface{}{
					"repoURL":        spec.RepoURL,
					"targetRevision": spec.TargetRevision,
					"path":           "{{path}}",
				},
				"destination": map[string]interface{}{
					"server":    spec.DestinationServer,
					"namespace": spec.DestinationNamespace,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    true,
						"selfHeal": true,
					},
				},
			},
		},
	}
	appSet.SetManagedFields(nil)
	if err != nil {
		return cl.Create(ctx, appSet)
	}
	return cl.Update(ctx, appSet)
}

func validateArgoCDScopedDestination(project, destinationNamespace string) error {
	if strings.EqualFold(project, "default") {
		return fmt.Errorf("argocd default project is disabled; use the environment project")
	}
	switch destinationNamespace {
	case "default", "kube-system", "kube-public", "kube-node-lease":
		return fmt.Errorf("destination namespace %q is reserved for cluster system workloads", destinationNamespace)
	}
	return nil
}

func DeleteArgoCDApplication(ctx context.Context, namespace, name string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	if namespace == "" || name == "" {
		return fmt.Errorf("namespace and name are required")
	}
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	app.SetNamespace(namespace)
	app.SetName(name)
	if err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, app); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if ensureObjectFinalizer(app, argoCDResourcesFinalizer) {
		if err := cl.Update(ctx, app); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}
	err = cl.Delete(ctx, app)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func ensureObjectFinalizer(obj client.Object, finalizer string) bool {
	for _, existing := range obj.GetFinalizers() {
		if existing == finalizer {
			return false
		}
	}
	obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))
	return true
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "Unknown"
	}
	return value
}
