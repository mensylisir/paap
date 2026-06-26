package controller

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	paapv1 "paap/api/v1"
)

func TestEnsureNetworkPolicyAllowsBusinessNamespacesToReachSharedResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &EnvironmentReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"paap.io/app": "billing"},
		},
		Spec: paapv1.EnvironmentSpec{Identifier: "prod"},
	}

	if err := r.ensureNetworkPolicy(context.Background(), env, "billing-prod"); err != nil {
		t.Fatalf("ensure network policy: %v", err)
	}

	var policy networkingv1.NetworkPolicy
	if err := r.Get(context.Background(), types.NamespacedName{Name: "paap-deny-cross-env", Namespace: "billing-prod"}, &policy); err != nil {
		t.Fatalf("get network policy: %v", err)
	}
	if !networkPolicyAllowsEgressToNamespace(policy, map[string]string{"paap.io/app": "default", "paap.io/env": "shared"}) {
		t.Fatalf("network policy egress should allow default/shared namespaces: %#v", policy.Spec.Egress)
	}
}

func networkPolicyAllowsEgressToNamespace(policy networkingv1.NetworkPolicy, labels map[string]string) bool {
	for _, rule := range policy.Spec.Egress {
		for _, peer := range rule.To {
			if peer.NamespaceSelector == nil {
				continue
			}
			matches := true
			for key, value := range labels {
				if peer.NamespaceSelector.MatchLabels[key] != value {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
	}
	return false
}
