//nolint:testpackage
package gateway

// import (
// 	"testing"

// 	v1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"sigs.k8s.io/controller-runtime/pkg/client/fake"

// 	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
// 	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
// 	odhtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"

// 	. "github.com/onsi/gomega"
// )

// func TestCreateGatewayInfrastructure(t *testing.T) {
// 	g := NewWithT(t)
// 	ctx := t.Context()

// 	gateway := &serviceApi.Gateway{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "default-gateway",
// 		},
// 		Spec: serviceApi.GatewaySpec{
// 			Domain:    "odh.example.com",
// 			Namespace: "openshift-ingress",
// 			Auth: serviceApi.GatewayAuthSpec{
// 				Mode: "auto",
// 			},
// 			Certificates: serviceApi.GatewayCertSpec{
// 				Type: "openshift-service-ca",
// 			},
// 		},
// 		Status: serviceApi.GatewayStatus{
// 			Status: common.Status{
// 				Conditions: []common.Condition{},
// 			},
// 		},
// 	}

// 	scheme := runtime.NewScheme()
// 	serviceApi.AddToScheme(scheme)
// 	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gateway).Build()

// 	rr := &odhtypes.ReconciliationRequest{
// 		Client:   client,
// 		Instance: gateway,
// 	}

// 	// Test the action
// 	err := createGatewayInfrastructure(ctx, rr)
// 	g.Expect(err).NotTo(HaveOccurred())

// 	// TODO: Add assertions for created resources
// 	// - Verify GatewayClass was created
// 	// - Verify Gateway was created in correct namespace
// 	// - Verify HTTPS listeners are configured
// }

// // Helper function to create test Gateway with minimal configuration
// func createTestGateway(name string) *serviceApi.Gateway {
// 	return &serviceApi.Gateway{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: name,
// 		},
// 		Spec: serviceApi.GatewaySpec{
// 			Domain:    "odh.example.com",
// 			Namespace: "openshift-ingress",
// 			Auth: serviceApi.GatewayAuthSpec{
// 				Mode: "auto",
// 			},
// 			Certificates: serviceApi.GatewayCertSpec{
// 				Type: "openshift-service-ca",
// 			},
// 		},
// 		Status: serviceApi.GatewayStatus{
// 			Status: common.Status{
// 				Conditions: []common.Condition{},
// 			},
// 		},
// 	}
// }

// // Helper function to create ReconciliationRequest for testing
// func createTestReconciliationRequest(gateway *serviceApi.Gateway) *odhtypes.ReconciliationRequest {
// 	scheme := runtime.NewScheme()
// 	serviceApi.AddToScheme(scheme)
// 	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gateway).Build()

// 	return &odhtypes.ReconciliationRequest{
// 		Client:   client,
// 		Instance: gateway,
// 	}
// }
