package main

import (
	"context"
	"log"
	"os"
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
	diskPath string = "/tmp/targetpvc/disk.img"
)

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

	image, err := buildContainerDisk(diskPath)
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
