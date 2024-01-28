package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	cobra "github.com/spf13/cobra"
	tar "kubevirt.io/containerdisks/pkg/build"
)

const (
	diskPath             string = "./tmp/disk.img.gz"
	diskPathDecompressed string = "./tmp/disk.img"
	diskPathConverted    string = "./tmp/disk.qcow2"
)

func applyVirtualMachineExport(vmName string) error {
	log.Println("Applying VirtualMachineExport object...")

	yaml := fmt.Sprintf(`
apiVersion: export.kubevirt.io/v1alpha1
kind: VirtualMachineExport
metadata:
  name: %s
spec:
  source:
    apiGroup: "kubevirt.io"
    kind: VirtualMachine
    name: %s
`, vmName, vmName)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
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

	cmd := exec.Command("gunzip", diskPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	log.Println("Decompressed completed successfully.")
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

func pushContainerDisk(image v1.Image, containerDiskName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	hostName := os.Getenv("REGISTRY_HOST")
	userName := os.Getenv("REGISTRY_USERNAME")
	password := os.Getenv("REGISTRY_PASSWORD")
	auth := &authn.Basic{
		Username: userName,
		Password: password,
	}

	imageDestination := fmt.Sprintf("%s/%s/%s", hostName, userName, containerDiskName)
	err := crane.Push(image, imageDestination, crane.WithAuth(auth), crane.WithContext(ctx))
	if err != nil {
		log.Fatalf("Error pushing image: %v", err)
		return err
	}

	log.Println("Image pushed successfully")
	return nil
}

func run(vmName, volumeName, containerDiskName, enableVirtSysprep string) error {
	if err := applyVirtualMachineExport(vmName); err != nil {
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

	return pushContainerDisk(image, containerDiskName)
}

func main() {
	var vmName string
	var volumeName string
	var containerDiskName string
	var enableVirtSysprep string

	var command = &cobra.Command{
		Use:   "kubevirt-disk-uploader",
		Short: "Extracts disk and uploads it to a container registry",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("Extracts disk and uploads it to a container registry...")

			if err := run(vmName, volumeName, containerDiskName, enableVirtSysprep); err != nil {
				log.Panicln(err)
			}

			log.Println("Succesfully extracted disk image and uploaded it in a new container image to container registry.")
		},
	}

	command.Flags().StringVar(&vmName, "vmname", "", "name of the virtual machine")
	command.Flags().StringVar(&volumeName, "volumename", "", "volume name of the virtual machine")
	command.Flags().StringVar(&containerDiskName, "containerdiskname", "", "name of the new container image")
	command.Flags().StringVar(&enableVirtSysprep, "enablevirtsysprep", "false", "enable or disable virt-sysprep")
	command.MarkFlagRequired("vmname")
	command.MarkFlagRequired("volumename")
	command.MarkFlagRequired("containerDiskName")

	if err := command.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
