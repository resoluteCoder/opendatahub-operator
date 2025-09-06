/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dscinitialization

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

// handleGateway manages Gateway infrastructure based on DSCI configuration
func (r *DSCInitializationReconciler) handleGateway(ctx context.Context, dsci *dsciv1.DSCInitialization) error {
	log := logf.FromContext(ctx).WithName("handleGateway")

	// Check if gateway configuration exists
	if dsci.Spec.Gateway == nil {
		log.Info("No gateway configuration in DSCI, skipping Gateway infrastructure")
		return nil
	}

	log.Info("Creating Gateway infrastructure")

	// Create Gateway infrastructure
	if err := r.createGatewayInfrastructure(ctx, dsci); err != nil {
		return fmt.Errorf("failed to create Gateway infrastructure: %w", err)
	}

	return nil
}

// createGatewayInfrastructure creates the Gateway API resources
func (r *DSCInitializationReconciler) createGatewayInfrastructure(ctx context.Context, dsci *dsciv1.DSCInitialization) error {
	log := logf.FromContext(ctx).WithName("createGatewayInfrastructure")

	// Create GatewayClass
	if err := r.createGatewayClass(ctx, dsci); err != nil {
		return fmt.Errorf("failed to create GatewayClass: %w", err)
	}

	// Handle certificates
	certSecretName, err := r.handleGatewayCertificates(ctx, dsci)
	if err != nil {
		return fmt.Errorf("failed to handle certificates: %w", err)
	}

	// Create Gateway
	if err := r.createGateway(ctx, dsci, certSecretName); err != nil {
		return fmt.Errorf("failed to create Gateway: %w", err)
	}

	log.Info("Successfully created Gateway infrastructure")

	return nil
}

// createGatewayClass creates the GatewayClass resource
func (r *DSCInitializationReconciler) createGatewayClass(ctx context.Context, dsci *dsciv1.DSCInitialization) error {
	gatewayClass := &gwapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "odh-gateway-class",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "opendatahub-operator",
				"opendatahub.io/internal":      "true",
			},
		},
		Spec: gwapiv1.GatewayClassSpec{
			ControllerName: "openshift.io/gateway-controller/v1",
		},
	}

	if err := controllerutil.SetOwnerReference(dsci, gatewayClass, r.Client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	return resources.Apply(
		ctx,
		r.Client,
		gatewayClass,
		client.FieldOwner(fieldManager),
		client.ForceOwnership,
	)
}

// handleGatewayCertificates manages certificate configuration
func (r *DSCInitializationReconciler) handleGatewayCertificates(ctx context.Context, dsci *dsciv1.DSCInitialization) (string, error) {
	// Always use cert-manager for Gateway certificates
	return r.createCertManagerCertificate(ctx, dsci)
}

// createCertManagerCertificate creates a cert-manager Certificate
func (r *DSCInitializationReconciler) createCertManagerCertificate(ctx context.Context, dsci *dsciv1.DSCInitialization) (string, error) {
	secretName := "odh-gateway-tls"
	gatewayNamespace := "openshift-ingress"

	domain, err := cluster.GetDomain(ctx, r.Client)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster domain: %w", err)
	}

	issuerRef := cmmeta.ObjectReference{
		Name: "selfsigned-cluster-issuer",
		Kind: "ClusterIssuer",
	}

	certificate := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-gateway-cert",
			Namespace: gatewayNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "opendatahub-operator",
				"opendatahub.io/internal":      "true",
			},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: secretName,
			DNSNames:   []string{"odh-gateway." + domain},
			IssuerRef:  issuerRef,
		},
	}

	if err := controllerutil.SetOwnerReference(dsci, certificate, r.Client.Scheme()); err != nil {
		return "", fmt.Errorf("failed to set owner reference: %w", err)
	}

	err = resources.Apply(
		ctx,
		r.Client,
		certificate,
		client.FieldOwner(fieldManager),
		client.ForceOwnership,
	)
	if err != nil {
		return "", fmt.Errorf("failed to apply Certificate resource: %w", err)
	}

	return secretName, nil
}

// createGateway creates the Gateway API Gateway resource
func (r *DSCInitializationReconciler) createGateway(ctx context.Context, dsci *dsciv1.DSCInitialization, certSecretName string) error {
	// Get cluster domain for Gateway configuration
	domain, err := cluster.GetDomain(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to get cluster domain: %w", err)
	}

	listeners := r.createGatewayListeners(certSecretName, domain)
	gatewayNamespace := "openshift-ingress"

	gateway := &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-gateway",
			Namespace: gatewayNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "opendatahub-operator",
				"opendatahub.io/internal":      "true",
			},
		},
		Spec: gwapiv1.GatewaySpec{
			GatewayClassName: "odh-gateway-class",
			Listeners:        listeners,
		},
	}

	if err := controllerutil.SetOwnerReference(dsci, gateway, r.Client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	return resources.Apply(
		ctx,
		r.Client,
		gateway,
		client.FieldOwner(fieldManager),
		client.ForceOwnership,
	)
}

// createGatewayListeners creates listeners for the Gateway
func (r *DSCInitializationReconciler) createGatewayListeners(certSecretName string, domain string) []gwapiv1.Listener {
	listeners := []gwapiv1.Listener{}

	if certSecretName != "" {
		httpsMode := gwapiv1.TLSModeTerminate
		hostname := gwapiv1.Hostname("odh-gateway." + domain)
		httpsListener := gwapiv1.Listener{
			Name:     "https",
			Protocol: gwapiv1.HTTPSProtocolType,
			Port:     443,
			Hostname: &hostname,
			TLS: &gwapiv1.GatewayTLSConfig{
				Mode: &httpsMode,
				CertificateRefs: []gwapiv1.SecretObjectReference{
					{
						Name: gwapiv1.ObjectName(certSecretName),
					},
				},
			},
		}
		listeners = append(listeners, httpsListener)
	}

	return listeners
}
