package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
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
	kvcorev1 "kubevirt.io/api/core/v1"
	v1beta1 "kubevirt.io/api/export/v1beta1"
	kubecli "kubevirt.io/client-go/kubecli"
	tar "kubevirt.io/containerdisks/pkg/build"
)

const (
	diskPath             string = "./tmp/disk.img.gz"
	diskPathDecompressed string = "./tmp/disk.img"
	diskPathConverted    string = "./tmp/disk.qcow2"
)

func applyVirtualMachineExport(vmNamespace, vmName string) error {
	log.Println("Applying VirtualMachineExport object...")

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

	_, err = client.VirtualMachineExport(vmNamespace).Create(context.Background(), vmExport, metav1.CreateOptions{})
	return err
}

func downloadVirtualMachineDiskImage(vmName, volumeName string) error {
	log.Printf("Downloading disk image from %s Virtual Machine...\n", vmName)

	cmd := exec.Command("usr/bin/virtctl", "vmexport", "download", vmName, "--vm", vmName, "--volume", volumeName, "--output", diskPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	if fileInfo, err := os.Stat(diskPath); err != nil || fileInfo.Size() == 0 {
		return fmt.Errorf("File does not exist or is empty.")
	}

	log.Println("Download completed successfully.")
	return nil
}

func decompressVirtualMachineDiskImage() error {
	log.Println("Decompressing downloaded disk image...")

	diskImg, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer diskImg.Close()

	gzipReader, err := gzip.NewReader(diskImg)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	newDiskImg, err := os.Create(diskPathDecompressed)
	if err != nil {
		return err
	}
	defer newDiskImg.Close()

	_, err = io.Copy(newDiskImg, gzipReader)
	if err != nil {
		return err
	}

	err = os.Chmod(diskPathDecompressed, 0666) // Grants read and write permission to everyone.
	if err != nil {
		return err
	}

	log.Println("Decompression completed successfully.")
	return nil
}

func convertRawDiskImageToQcow2() error {
	log.Println("Converting raw disk image to qcow2 format...")

	cmd := exec.Command("qemu-img", "convert", "-f", "raw", "-O", "qcow2", diskPathDecompressed, diskPathConverted)
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

func prepareVirtualMachineDiskImage(enableVirtSysprep string) error {
	enabled, err := strconv.ParseBool(enableVirtSysprep)
	if err != nil {
		return err
	}

	if !enabled {
		log.Println("Skipping disk image preparation.")
		return nil
	}

	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd := exec.Command("virt-sysprep", "--format", "qcow2", "-a", diskPathConverted)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

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

func run(vmNamespace, vmName, volumeName, imageDestination, enableVirtSysprep string, pushTimeout int) error {
	if err := applyVirtualMachineExport(vmNamespace, vmName); err != nil {
		return err
	}

	if err := downloadVirtualMachineDiskImage(vmName, volumeName); err != nil {
		return err
	}

	if err := decompressVirtualMachineDiskImage(); err != nil {
		return err
	}

	if err := convertRawDiskImageToQcow2(); err != nil {
		return err
	}

	if err := prepareVirtualMachineDiskImage(enableVirtSysprep); err != nil {
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
	var enableVirtSysprep string
	var pushTimeout int

	var command = &cobra.Command{
		Use:   "kubevirt-disk-uploader",
		Short: "Extracts disk and uploads it to a container registry",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("Extracts disk and uploads it to a container registry...")

			if err := run(vmNamespace, vmName, volumeName, imageDestination, enableVirtSysprep, pushTimeout); err != nil {
				log.Panicln(err)
			}

			log.Println("Succesfully extracted disk image and uploaded it in a new container image to container registry.")
		},
	}

	command.Flags().StringVar(&vmNamespace, "vmnamespace", "", "namespace of the virtual machine")
	command.Flags().StringVar(&vmName, "vmname", "", "name of the virtual machine")
	command.Flags().StringVar(&volumeName, "volumename", "", "volume name of the virtual machine")
	command.Flags().StringVar(&imageDestination, "imagedestination", "", "destination of the image in container registry")
	command.Flags().StringVar(&enableVirtSysprep, "enablevirtsysprep", "false", "enable or disable virt-sysprep")
	command.Flags().IntVar(&pushTimeout, "pushtimeout", 60, "containerdisk push timeout in minutes")
	command.MarkFlagRequired("vmname")
	command.MarkFlagRequired("volumename")
	command.MarkFlagRequired("imagedestination")

	if err := command.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
