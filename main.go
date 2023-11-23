package main

import (
	"context"
	"fmt"
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

func build(diskPath string) (v1.Image, error) {
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

func push(image v1.Image, imageDestination string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()

	username := os.Getenv("REGISTRY_USERNAME")
	password := os.Getenv("REGISTRY_PASSWORD")
	auth := &authn.Basic{
		Username: username,
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

func main() {
	var diskPath string
	var imageDestination string

	var command = &cobra.Command{
		Use:   "kubevirt-disk-uploader",
		Short: "Extracts disk and uploads it to a container registry",
		Run: func(cmd *cobra.Command, args []string) {
			if image, err := build(diskPath); err == nil {
				push(image, imageDestination)
			}
		},
	}

	command.Flags().StringVarP(&diskPath, "diskpath", "d", "", "path to the disk")
	command.Flags().StringVarP(&imageDestination, "imagedestination", "i", "", "destination of the image")
	command.MarkFlagRequired("diskpath")
	command.MarkFlagRequired("imagedestination")

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
