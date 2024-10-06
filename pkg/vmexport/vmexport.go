package vmexport

import (
	"context"
	"fmt"
	"time"

	"github.com/codingben/kubevirt-disk-uploader/pkg/ownerreference"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	kvcorev1 "kubevirt.io/api/core/v1"
	v1beta1 "kubevirt.io/api/export/v1beta1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
	kubecli "kubevirt.io/client-go/kubecli"
)

const (
	sourceVM         string = "vm"
	sourceVMSnapshot string = "vmsnapshot"
	sourcePVC        string = "pvc"
)

var (
	exportSources = map[string]struct{}{sourceVM: {}, sourceVMSnapshot: {}, sourcePVC: {}}
)

func CreateVirtualMachineExport(client kubecli.KubevirtClient, exportSourceKind, exportSourceNamespace, exportSourceName string) error {
	if !isValidExportSource(exportSourceKind) {
		return fmt.Errorf("invalid export-source-kind: %s, must be one of vm, vmsnapshot, pvc", exportSourceKind)
	}

	source, err := getExportSource(exportSourceKind, exportSourceName)
	if err != nil {
		return err
	}

	v1VmExport := &v1beta1.VirtualMachineExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exportSourceName,
			Namespace: exportSourceNamespace,
		},
		Spec: v1beta1.VirtualMachineExportSpec{
			TokenSecretRef: &exportSourceName,
			Source:         source,
		},
	}

	if err := ownerreference.SetPodOwnerReference(client, v1VmExport); err != nil {
		return err
	}

	_, err = client.VirtualMachineExport(exportSourceNamespace).Create(context.Background(), v1VmExport, metav1.CreateOptions{})
	return err
}

func WaitUntilVirtualMachineExportReady(client kubecli.KubevirtClient, namespace, name string) error {
	pollInterval := 15 * time.Second
	pollTimeout := 3600 * time.Second
	poller := func(ctx context.Context) (bool, error) {
		vmExport, err := client.VirtualMachineExport(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if vmExport.Status != nil && vmExport.Status.Phase == v1beta1.Ready {
			return true, nil
		}
		return false, nil
	}

	return wait.PollUntilContextTimeout(context.Background(), pollInterval, pollTimeout, true, poller)
}

func GetRawDiskUrlFromVolumes(client kubecli.KubevirtClient, namespace, name, volumeName string) (string, error) {
	vmExport, err := client.VirtualMachineExport(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if vmExport.Status.Links == nil && vmExport.Status.Links.Internal == nil {
		return "", fmt.Errorf("no links found in VirtualMachineExport status")
	}

	for _, volume := range vmExport.Status.Links.Internal.Volumes {
		if volumeName != volume.Name {
			continue
		}

		for _, format := range volume.Formats {
			if format.Format == v1beta1.KubeVirtRaw {
				return format.Url, nil
			}
		}
	}
	return "", fmt.Errorf("volume %s is not found in VirtualMachineExport internal volumes", volumeName)
}

func getExportSource(exportSourceKind, exportSourceName string) (corev1.TypedLocalObjectReference, error) {
	switch exportSourceKind {
	case sourceVM:
		return corev1.TypedLocalObjectReference{
			APIGroup: &kvcorev1.SchemeGroupVersion.Group,
			Kind:     "VirtualMachine",
			Name:     exportSourceName,
		}, nil
	case sourceVMSnapshot:
		return corev1.TypedLocalObjectReference{
			APIGroup: &snapshotv1.SchemeGroupVersion.Group,
			Kind:     "VirtualMachineSnapshot",
			Name:     exportSourceName,
		}, nil
	case sourcePVC:
		return corev1.TypedLocalObjectReference{
			APIGroup: &corev1.SchemeGroupVersion.Group,
			Kind:     "PersistentVolumeClaim",
			Name:     exportSourceName,
		}, nil
	default:
		return corev1.TypedLocalObjectReference{}, fmt.Errorf("invalid export-source-kind: %s", exportSourceKind)
	}
}

func isValidExportSource(exportSourceKind string) bool {
	if _, ok := exportSources[exportSourceKind]; ok {
		return true
	}
	return false
}
