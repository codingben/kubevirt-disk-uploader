package main

import (
	"log"
	"os"

	"github.com/codingben/kubevirt-disk-uploader/pkg/certificate"
	"github.com/codingben/kubevirt-disk-uploader/pkg/disk"
	"github.com/codingben/kubevirt-disk-uploader/pkg/image"
	"github.com/codingben/kubevirt-disk-uploader/pkg/secrets"
	"github.com/codingben/kubevirt-disk-uploader/pkg/vmexport"

	cobra "github.com/spf13/cobra"
	kubecli "kubevirt.io/client-go/kubecli"
)

const (
	diskPath            string = "./tmp/disk.qcow2"
	certificatePath     string = "./tmp/tls.crt"
	kvExportTokenHeader string = "x-kubevirt-export-token"
)

func run(client kubecli.KubevirtClient, vmNamespace, vmName, volumeName, imageDestination string, pushTimeout int) error {
	log.Printf("Creating a new Secret '%s/%s' object...", vmNamespace, vmName)

	if err := secrets.CreateVirtualMachineExportSecret(client, vmNamespace, vmName); err != nil {
		return err
	}

	log.Printf("Creating a new VirtualMachineExport '%s/%s' object...", vmNamespace, vmName)

	if err := vmexport.CreateVirtualMachineExport(client, vmNamespace, vmName); err != nil {
		return err
	}

	log.Println("Waiting for VirtualMachineExport status to be ready...")

	if err := vmexport.WaitUntilVirtualMachineExportReady(client, vmNamespace, vmName); err != nil {
		return err
	}

	log.Println("Getting raw disk URL from the VirtualMachineExport object status...")

	rawDiskUrl, err := vmexport.GetRawDiskUrlFromVolumes(client, vmNamespace, vmName, volumeName)
	if err != nil {
		return err
	}

	log.Println("Creating TLS certificate file from the VirtualMachineExport object status...")

	certificateData, err := certificate.GetCertificateFromVirtualMachineExport(client, vmNamespace, vmName)
	if err != nil {
		return err
	}

	if err := certificate.CreateCertificateFile(certificatePath, certificateData); err != nil {
		return err
	}

	log.Println("Getting export token from the Secret object...")

	kvExportToken, err := secrets.GetTokenFromVirtualMachineExportSecret(client, vmNamespace, vmName)
	if err != nil {
		return err
	}

	log.Println("Downloading disk image from the VirtualMachineExport server...")

	if err := disk.DownloadDiskImageFromURL(rawDiskUrl, kvExportTokenHeader, kvExportToken, certificatePath, diskPath); err != nil {
		return err
	}

	log.Println("Building a new container image...")

	containerImage, err := image.Build(diskPath)
	if err != nil {
		return err
	}

	log.Println("Pushing new container image to the container registry...")

	if err := image.Push(containerImage, imageDestination, pushTimeout); err != nil {
		return err
	}

	log.Println("Successfully uploaded to the container registry.")
	return nil
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
			client, err := kubecli.GetKubevirtClient()
			if err != nil {
				log.Panicln(err)
			}

			namespace := os.Getenv("POD_NAMESPACE")
			if namespace != "" {
				vmNamespace = namespace
			}

			if err := run(client, namespace, vmName, volumeName, imageDestination, pushTimeout); err != nil {
				log.Panicln(err)
			}
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
