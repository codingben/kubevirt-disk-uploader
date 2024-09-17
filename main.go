package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	cobra "github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kvcorev1 "kubevirt.io/api/core/v1"
	v1beta1 "kubevirt.io/api/export/v1beta1"
	kubecli "kubevirt.io/client-go/kubecli"
	tar "kubevirt.io/containerdisks/pkg/build"
)

const (
	pollInterval                = 15 * time.Second
	pollTimeout                 = 3600 * time.Second
	diskPath             string = "./tmp/disk.img.gz"
	diskPathDecompressed string = "./tmp/disk.img"
	diskPathConverted    string = "./tmp/disk.qcow2"
)

func applyVirtualMachineExport(client kubecli.KubevirtClient, vmNamespace, vmName string) error {
	log.Println("Applying VirtualMachineExport object...")

	vmExport := &v1beta1.VirtualMachineExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: vmNamespace,
		},
		Spec: v1beta1.VirtualMachineExportSpec{
			Source: corev1.TypedLocalObjectReference{
				APIGroup: &kvcorev1.GroupVersion.Version,
				Kind:     kvcorev1.VirtualMachineGroupVersionKind.Kind,
				Name:     vmName,
			},
		},
	}

	_, err := client.VirtualMachineExport(vmNamespace).Create(context.Background(), vmExport, metav1.CreateOptions{})
	return err
}

func getRawDiskUrlFromVirtualMachineExport(client kubecli.KubevirtClient, vmNamespace, vmName, volumeName string) (string, error) {
	log.Println("Waiting for VirtualMachineExport to be ready...")

	vmExport, err := getVirtualMachineExportOnceReady(client, vmNamespace, vmName)
	if err != nil {
		return "", err
	}

	if vmExport.Status.Links == nil && vmExport.Status.Links.Internal == nil {
		return "", fmt.Errorf("No links found in VirtualMachineExport status.")
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
	return "", fmt.Errorf("Could not get raw disk URL from the VirtualMachineExport object.")
}

func getVirtualMachineExportOnceReady(client kubecli.KubevirtClient, vmNamespace, vmName string) (*v1beta1.VirtualMachineExport, error) {
	var vmExport *v1beta1.VirtualMachineExport

	poller := func(ctx context.Context) (bool, error) {
		vmExport, err := client.VirtualMachineExport(vmNamespace).Get(ctx, vmName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if vmExport.Status.Phase == v1beta1.Ready {
			return true, nil
		}
		return false, nil
	}

	err := wait.PollUntilContextTimeout(context.Background(), pollInterval, pollTimeout, true, poller)
	if err != nil {
		return nil, fmt.Errorf("Failed to wait for VirtualMachineExport to be ready: %v", err)
	}
	return vmExport, nil
}

func convertRawDiskImageToQcow2(rawDiskUrl string) error {
	log.Println("Converting raw disk image to qcow2 format...")

	cmd := exec.Command(
		"nbdkit",
		"-r",
		"curl",
		rawDiskUrl,
		"--run",
		fmt.Sprintf("qemu-img convert \"$uri\" -O qcow2 %s", diskPathConverted),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	if fileInfo, err := os.Stat(diskPathConverted); err != nil || fileInfo.Size() == 0 {
		return fmt.Errorf("Converted file does not exist or is empty.")
	}

	log.Println("Conversion to qcow2 format completed successfully.")
	return nil
}

func buildContainerDisk(diskPath string) (v1.Image, error) {
	layer, err := tarball.LayerFromOpener(tar.StreamLayerOpener(diskPath))
	if err != nil {
		log.Fatalf("Error creating layer from file: %v", err)
		return nil, err
	}

	image, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		log.Fatalf("Error appending layer: %v", err)
		return nil, err
	}

	log.Println("Image built successfully", image)
	return image, nil
}

func pushContainerDisk(image v1.Image, imageDestination string, pushTimeout int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(pushTimeout))
	defer cancel()

	userName := os.Getenv("REGISTRY_USERNAME")
	password := os.Getenv("REGISTRY_PASSWORD")
	auth := &authn.Basic{
		Username: userName,
		Password: password,
	}

	err := crane.Push(image, imageDestination, crane.WithAuth(auth), crane.WithContext(ctx))
	if err != nil {
		log.Fatalf("Error pushing image: %v", err)
		return err
	}

	log.Println("Image pushed successfully")
	return nil
}

func run(vmNamespace, vmName, volumeName, imageDestination string, pushTimeout int) error {
	client, err := kubecli.GetKubevirtClient()
	if err != nil {
		return err
	}

	env := os.Getenv("VM_NAMESPACE")
	if env != "" {
		vmNamespace = env
	}

	if vmNamespace == "" {
		return fmt.Errorf("VM namespace is not defined. Set VM_NAMESPACE or parameter.")
	}

	if err := applyVirtualMachineExport(client, vmNamespace, vmName); err != nil {
		return err
	}

	rawDiskUrl, err := getRawDiskUrlFromVirtualMachineExport(client, vmNamespace, vmName, volumeName)
	if err != nil {
		return err
	}

	if err := convertRawDiskImageToQcow2(rawDiskUrl); err != nil {
		return err
	}

	image, err := buildContainerDisk(diskPathConverted)
	if err != nil {
		return err
	}

	return pushContainerDisk(image, imageDestination, pushTimeout)
}

func main() {
	var vmNamespace string
	var vmName string
	var volumeName string
	var imageDestination string
	var pushTimeout int

	var command = &cobra.Command{
		Use:   "kubevirt-disk-uploader",
		Short: "Extracts disk and uploads it to a container registry",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("Extracts disk and uploads it to a container registry...")

			if err := run(vmNamespace, vmName, volumeName, imageDestination, pushTimeout); err != nil {
				log.Panicln(err)
			}

			log.Println("Succesfully extracted disk image and uploaded it in a new container image to container registry.")
		},
	}

	command.Flags().StringVar(&vmNamespace, "vmnamespace", "", "namespace of the virtual machine")
	command.Flags().StringVar(&vmName, "vmname", "", "name of the virtual machine")
	command.Flags().StringVar(&volumeName, "volumename", "", "volume name of the virtual machine")
	command.Flags().StringVar(&imageDestination, "imagedestination", "", "destination of the image in container registry")
	command.Flags().IntVar(&pushTimeout, "pushtimeout", 60, "containerdisk push timeout in minutes")
	command.MarkFlagRequired("vmname")
	command.MarkFlagRequired("volumename")
	command.MarkFlagRequired("imagedestination")

	if err := command.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
