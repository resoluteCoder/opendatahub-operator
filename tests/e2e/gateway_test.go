package e2e_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	. "github.com/onsi/gomega"
)

const (
	gatewayName          = "odh-gateway"
	gatewayClassName     = "odh-gateway-class"
	gatewayNamespace     = "openshift-ingress"
	gatewayCertName      = "odh-gateway-cert"
	gatewayTLSSecretName = "odh-gateway-tls"
)

var (
	GatewayClassGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "GatewayClass",
	}

	GatewayGVK = schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "Gateway",
	}

	CertificateGVK = schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
	}
)

func gatewayTestSuite(t *testing.T) {
	ctx, err := NewTestContext(t)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Gateway infrastructure validation", func(t *testing.T) {
		validateGatewayCreation(t, ctx)
	})
}

func validateGatewayCreation(t *testing.T, ctx *TestContext) {
	t.Helper()

	t.Log("Validating Gateway API resources creation")

	t.Run("GatewayClass should be created", func(t *testing.T) {
		ctx.EnsureResourceExists(
			WithMinimalObject(GatewayClassGVK, types.NamespacedName{Name: gatewayClassName}),
		)
	})

	t.Run("Certificate should be created", func(t *testing.T) {
		ctx.EnsureResourceExists(
			WithMinimalObject(CertificateGVK, types.NamespacedName{
				Name:      gatewayCertName,
				Namespace: gatewayNamespace,
			}),
		)
	})

	t.Run("Gateway should be created with correct configuration", func(t *testing.T) {
		ctx.EnsureResourceExists(
			WithMinimalObject(GatewayGVK, types.NamespacedName{
				Name:      gatewayName,
				Namespace: gatewayNamespace,
			}),
		)

		gateway := &gwapiv1.Gateway{}
		Eventually(func() error {
			if err := ctx.Client().Get(ctx.Context(), types.NamespacedName{
				Name:      gatewayName,
				Namespace: gatewayNamespace,
			}, gateway); err != nil {
				return err
			}

			if gateway.Spec.GatewayClassName != gatewayClassName {
				t.Errorf("Expected GatewayClassName %s, got %s", gatewayClassName, gateway.Spec.GatewayClassName)
			}

			if len(gateway.Spec.Listeners) == 0 {
				t.Error("Gateway should have at least one listener")
			}

			httpsListener := findListenerByName(gateway.Spec.Listeners, "https")
			if httpsListener == nil {
				t.Error("Gateway should have an HTTPS listener")
				return nil
			}

			if httpsListener.Protocol != gwapiv1.HTTPSProtocolType {
				t.Errorf("Expected HTTPS protocol, got %s", httpsListener.Protocol)
			}

			if httpsListener.Port != 443 {
				t.Errorf("Expected port 443, got %d", httpsListener.Port)
			}

			if httpsListener.TLS == nil {
				t.Error("HTTPS listener should have TLS configuration")
				return nil
			}

			if len(httpsListener.TLS.CertificateRefs) == 0 {
				t.Error("HTTPS listener should have certificate references")
				return nil
			}

			expectedSecretName := gwapiv1.ObjectName(gatewayTLSSecretName)
			if httpsListener.TLS.CertificateRefs[0].Name != expectedSecretName {
				t.Errorf("Expected certificate secret name %s, got %s",
					expectedSecretName, httpsListener.TLS.CertificateRefs[0].Name)
			}

			return nil
		}, ctx.TestTimeouts.longEventuallyTimeout, ctx.TestTimeouts.defaultEventuallyPollInterval).Should(Succeed())
	})

	t.Log("Gateway API resources validation completed successfully")
}

func findListenerByName(listeners []gwapiv1.Listener, name string) *gwapiv1.Listener {
	for i := range listeners {
		if string(listeners[i].Name) == name {
			return &listeners[i]
		}
	}
	return nil
}
