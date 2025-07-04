package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"maps"

	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	infraAPI "github.com/opendatahub-io/opendatahub-operator/v2/api/infrastructure/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	odhtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhdeploy "github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

func initialize(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	rr.Manifests = []odhtypes.ManifestInfo{defaultManifestInfo(rr.Release.Name)}

	return nil
}

func devFlags(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	dashboard, ok := rr.Instance.(*componentApi.Dashboard)
	if !ok {
		return fmt.Errorf("resource instance %v is not a componentApi.Dashboard)", rr.Instance)
	}

	if dashboard.Spec.DevFlags == nil {
		return nil
	}
	// Implement devflags support logic
	// If dev flags are set, update default manifests path
	if len(dashboard.Spec.DevFlags.Manifests) != 0 {
		manifestConfig := dashboard.Spec.DevFlags.Manifests[0]
		if err := odhdeploy.DownloadManifests(ctx, ComponentName, manifestConfig); err != nil {
			return err
		}
		if manifestConfig.SourcePath != "" {
			rr.Manifests[0].Path = odhdeploy.DefaultManifestPath
			rr.Manifests[0].ContextDir = ComponentName
			rr.Manifests[0].SourcePath = manifestConfig.SourcePath
		}
	}

	return nil
}

func customizeResources(_ context.Context, rr *odhtypes.ReconciliationRequest) error {
	for i := range rr.Resources {
		if rr.Resources[i].GroupVersionKind() == gvk.OdhDashboardConfig {
			// mark the resource as not supposed to be managed by the operator
			resources.SetAnnotation(&rr.Resources[i], annotations.ManagedByODHOperator, "false")
			break
		}
	}

	return nil
}

func setKustomizedParams(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	extraParamsMap, err := computeKustomizeVariable(ctx, rr.Client, rr.Release.Name, &rr.DSCI.Spec)
	if err != nil {
		return errors.New("failed to set variable for url, section-title etc")
	}

	if err := odhdeploy.ApplyParams(rr.Manifests[0].String(), nil, extraParamsMap); err != nil {
		return fmt.Errorf("failed to update params.env from %s : %w", rr.Manifests[0].String(), err)
	}
	return nil
}

func configureDependencies(_ context.Context, rr *odhtypes.ReconciliationRequest) error {
	if rr.Release.Name == cluster.OpenDataHub {
		return nil
	}

	err := rr.AddResources(&corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "anaconda-ce-access",
			Namespace: rr.DSCI.Spec.ApplicationsNamespace,
		},
		Type: corev1.SecretTypeOpaque,
	})

	if err != nil {
		return fmt.Errorf("failed to create access-secret for anaconda: %w", err)
	}

	return nil
}

func updateStatus(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	d, ok := rr.Instance.(*componentApi.Dashboard)
	if !ok {
		return errors.New("instance is not of type *odhTypes.Dashboard")
	}

	// url
	rl := routev1.RouteList{}
	err := rr.Client.List(
		ctx,
		&rl,
		client.InNamespace(rr.DSCI.Spec.ApplicationsNamespace),
		client.MatchingLabels(map[string]string{
			labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	d.Status.URL = ""
	if len(rl.Items) == 1 {
		d.Status.URL = resources.IngressHost(rl.Items[0])
	}

	return nil
}
func reconcileHardwareProfiles(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	dashboardHardwareProfiles := &unstructured.UnstructuredList{}
	dashboardHardwareProfiles.SetGroupVersionKind(gvk.DashboardHardwareProfile.GroupVersion().WithKind("HardwareProfileList"))

	err := rr.Client.List(ctx, dashboardHardwareProfiles)
	if err != nil {
		return fmt.Errorf("failed to list dashboard hardware profiles: %w", err)
	}

	logger := log.FromContext(ctx)
	for _, hwprofile := range dashboardHardwareProfiles.Items {
		var dashboardHardwareProfile infraAPI.DashboardHardwareProfile

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(hwprofile.Object, &dashboardHardwareProfile); err != nil {
			return fmt.Errorf("failed to convert dashboard hardware profile: %w", err)
		}
		logger.Info(fmt.Sprintf("[HERE] dashboard item %s - %s", dashboardHardwareProfile.Namespace, dashboardHardwareProfile.Name))

		infraHWP := &infraAPI.HardwareProfile{}
		err := rr.Client.Get(ctx, client.ObjectKey{
			Name:      dashboardHardwareProfile.Name,
			Namespace: dashboardHardwareProfile.Namespace,
		}, infraHWP)

		if k8serr.IsNotFound(err) {
			logger.Info("[HERE] did not find anything")
			if err = createInfraHardwareProfile(ctx, rr, logger, &dashboardHardwareProfile); err != nil {
				return fmt.Errorf("failed to create infrastructure hardware profile: %w", err)
			}
			continue
		}

		logger.Info(fmt.Sprintf("[HERE] found %s - %s", infraHWP.Namespace, infraHWP.Name))

		if err != nil {
			return fmt.Errorf("failed to get infrastructure hardware profile: %w", err)
		}

		err = updateInfraHardwareProfile(ctx, rr, logger, &dashboardHardwareProfile, infraHWP)
		if err != nil {
			return fmt.Errorf("failed to update existing infrastructure hardware profile: %w", err)
		}

	}

	return nil
}

func createInfraHardwareProfile(ctx context.Context, rr *odhtypes.ReconciliationRequest, logger logr.Logger, dashboardhwprofile *infraAPI.DashboardHardwareProfile) error {
	annotations := make(map[string]string)
	maps.Copy(annotations, dashboardhwprofile.Annotations)

	annotations["opendatahub.io/migrated-from"] = fmt.Sprintf("hardwareprofiles.dashboard.opendatahub.io/%s", dashboardhwprofile.Name)
	annotations["opendatahub.io/display-name"] = dashboardhwprofile.Spec.DisplayName
	annotations["opendatahub.io/description"] = dashboardhwprofile.Spec.Description
	annotations["opendatahub.io/disabled"] = strconv.FormatBool(!dashboardhwprofile.Spec.Enabled)

	infraHardwareProfile := &infraAPI.HardwareProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dashboardhwprofile.Name,
			Namespace:   dashboardhwprofile.Namespace,
			Annotations: annotations,
		},
		Spec: infraAPI.HardwareProfileSpec{
			SchedulingSpec: &infraAPI.SchedulingSpec{
				SchedulingType: infraAPI.NodeScheduling,
				Node: &infraAPI.NodeSchedulingSpec{
					NodeSelector: dashboardhwprofile.Spec.NodeSelector,
					Tolerations:  dashboardhwprofile.Spec.Tolerations,
				},
			},
			Identifiers: dashboardhwprofile.Spec.Identifiers,
		},
	}

	if err := rr.Client.Create(ctx, infraHardwareProfile); err != nil {
		return err
	}

	logger.Info("succesfully created infrastructure hardware profile", "name", infraHardwareProfile.GetName())
	return nil
}

func updateInfraHardwareProfile(ctx context.Context, rr *odhtypes.ReconciliationRequest, logger logr.Logger, dashboardhwprofile *infraAPI.DashboardHardwareProfile, infrahwprofile *infraAPI.HardwareProfile) error {
	if infrahwprofile.Annotations == nil {
		infrahwprofile.Annotations = make(map[string]string)
	}

	maps.Copy(infrahwprofile.Annotations, dashboardhwprofile.Annotations)

	infrahwprofile.Annotations["opendatahub.io/migrated-from"] = fmt.Sprintf("hardwareprofiles.dashboard.opendatahub.io/%s", dashboardhwprofile.Name)
	infrahwprofile.Annotations["opendatahub.io/display-name"] = dashboardhwprofile.Spec.DisplayName
	infrahwprofile.Annotations["opendatahub.io/description"] = dashboardhwprofile.Spec.Description
	infrahwprofile.Annotations["opendatahub.io/disabled"] = strconv.FormatBool(!dashboardhwprofile.Spec.Enabled)

	infrahwprofile.Spec.SchedulingSpec = &infraAPI.SchedulingSpec{
		SchedulingType: infraAPI.NodeScheduling,
		Node: &infraAPI.NodeSchedulingSpec{
			NodeSelector: dashboardhwprofile.Spec.NodeSelector,
			Tolerations:  dashboardhwprofile.Spec.Tolerations,
		},
	}
	infrahwprofile.Spec.Identifiers = dashboardhwprofile.Spec.Identifiers

	if err := rr.Client.Update(ctx, infrahwprofile); err != nil {
		return fmt.Errorf("failed to update infrastructure hardware profile: %w", err)
	}

	logger.Info("successfully updated infrastructure hardware profile", "name", infrahwprofile.GetName())
	return nil
}

// func reconcileInfraHardwareProfile(ctx context.Context, rr *odhtypes.ReconciliationRequest, logger logr.Logger, dashboardhwprofile *infraAPI.DashboardHardwareProfile) error {
// 	infraHardwareProfile := &infraAPI.HardwareProfile{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      dashboardhwprofile.Name,
// 			Namespace: dashboardhwprofile.Namespace,
// 			Annotations: map[string]string{
// 				"opendatahub.io/migrated-from": fmt.Sprintf("hardwareprofiles.dashboard.opendatahub.io/%s", dashboardhwprofile.Name),
// 				"opendatahub.io/display-name":  dashboardhwprofile.Spec.DisplayName,
// 				"opendatahub.io/description":   dashboardhwprofile.Spec.Description,
// 				"opendatahub.io/disabled":      strconv.FormatBool(!dashboardhwprofile.Spec.Enabled),
// 			},
// 		},
// 		Spec: infraAPI.HardwareProfileSpec{
// 			SchedulingSpec: &infraAPI.SchedulingSpec{
// 				SchedulingType: infraAPI.NodeScheduling,
// 				Node: &infraAPI.NodeSchedulingSpec{
// 					NodeSelector: dashboardhwprofile.Spec.NodeSelector,
// 					Tolerations:  dashboardhwprofile.Spec.Tolerations,
// 				},
// 			},
// 			Identifiers: dashboardhwprofile.Spec.Identifiers,
// 		},
// 	}
// 	_, err := controllerutil.CreateOrUpdate(ctx, rr.Client, infraHardwareProfile, func() error {
// 		return nil
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
