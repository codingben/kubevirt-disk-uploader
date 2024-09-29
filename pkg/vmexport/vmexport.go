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
	kubecli "kubevirt.io/client-go/kubecli"
)

func CreateVirtualMachineExport(client kubecli.KubevirtClient, vmNamespace, vmName string) error {
	v1VmExport := &v1beta1.VirtualMachineExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: vmNamespace,
		},
		Spec: v1beta1.VirtualMachineExportSpec{
			TokenSecretRef: &vmName,
			Source: corev1.TypedLocalObjectReference{
				APIGroup: &kvcorev1.SchemeGroupVersion.Group,
				Kind:     kvcorev1.VirtualMachineGroupVersionKind.Kind,
				Name:     vmName,
			},
		},
	}

	if err := ownerreference.SetPodOwnerReference(client, v1VmExport); err != nil {
		return err
	}

	_, err := client.VirtualMachineExport(vmNamespace).Create(context.Background(), v1VmExport, metav1.CreateOptions{})
	return err
}

func WaitUntilVirtualMachineExportReady(client kubecli.KubevirtClient, vmNamespace, vmName string) error {
	pollInterval := 15 * time.Second
	pollTimeout := 3600 * time.Second
	poller := func(ctx context.Context) (bool, error) {
		vmExport, err := client.VirtualMachineExport(vmNamespace).Get(ctx, vmName, metav1.GetOptions{})
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

func GetRawDiskUrlFromVolumes(client kubecli.KubevirtClient, vmNamespace, vmName, volumeName string) (string, error) {
	vmExport, err := client.VirtualMachineExport(vmNamespace).Get(context.Background(), vmName, metav1.GetOptions{})
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
