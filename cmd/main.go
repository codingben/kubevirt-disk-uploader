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

type RunOptions struct {
	client                kubecli.KubevirtClient
	exportSourceKind      string
	exportSourceNamespace string
	exportSourceName      string
	volumeName            string
	imageDestination      string
	pushTimeout           int
}

func run(opts RunOptions) error {
	client := opts.client
	kind := opts.exportSourceKind
	name := opts.exportSourceName
	namespace := opts.exportSourceNamespace
	volumeName := opts.volumeName
	imageDestination := opts.imageDestination
	imagePushTimeout := opts.pushTimeout

	log.Printf("Creating a new Secret '%s/%s' object...", namespace, name)

	if err := secrets.CreateVirtualMachineExportSecret(client, namespace, name); err != nil {
		return err
	}

	log.Printf("Creating a new VirtualMachineExport '%s/%s' object...", namespace, name)

	if err := vmexport.CreateVirtualMachineExport(client, kind, namespace, name); err != nil {
		return err
	}

	log.Println("Waiting for VirtualMachineExport status to be ready...")

	if err := vmexport.WaitUntilVirtualMachineExportReady(client, namespace, name); err != nil {
		return err
	}

	log.Println("Getting raw disk URL from the VirtualMachineExport object status...")

	rawDiskUrl, err := vmexport.GetRawDiskUrlFromVolumes(client, namespace, name, volumeName)
	if err != nil {
		return err
	}

	log.Println("Creating TLS certificate file from the VirtualMachineExport object status...")

	certificateData, err := certificate.GetCertificateFromVirtualMachineExport(client, namespace, name)
	if err != nil {
		return err
	}

	if err := certificate.CreateCertificateFile(certificatePath, certificateData); err != nil {
		return err
	}

	log.Println("Getting export token from the Secret object...")

	kvExportToken, err := secrets.GetTokenFromVirtualMachineExportSecret(client, namespace, name)
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

	if err := image.Push(containerImage, imageDestination, imagePushTimeout); err != nil {
		return err
	}

	log.Println("Successfully uploaded to the container registry.")
	return nil
}

func main() {
	var opts RunOptions
	var command = &cobra.Command{
		Use:   "kubevirt-disk-uploader",
		Short: "Extracts disk and uploads it to a container registry",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := kubecli.GetKubevirtClient()
			if err != nil {
				log.Panicln(err)
			}
			opts.client = client

			namespace := os.Getenv("POD_NAMESPACE")
			if namespace != "" {
				opts.exportSourceNamespace = namespace
			}

			if err := run(opts); err != nil {
				log.Panicln(err)
			}
		},
	}

	command.Flags().StringVar(&opts.exportSourceKind, "export-source-kind", "", "specify the export source kind (vm, vmsnapshot, pvc)")
	command.Flags().StringVar(&opts.exportSourceNamespace, "export-source-namespace", "", "namespace of the export source")
	command.Flags().StringVar(&opts.exportSourceName, "export-source-name", "", "name of the export source")
	command.Flags().StringVar(&opts.volumeName, "volumename", "", "name of the volume (if source kind is 'pvc', then volume name is equal to source name)")
	command.Flags().StringVar(&opts.imageDestination, "imagedestination", "", "destination of the image in container registry")
	command.Flags().IntVar(&opts.pushTimeout, "pushtimeout", 60, "push timeout of container disk to registry")
	command.MarkFlagRequired("export-source-kind")
	command.MarkFlagRequired("export-source-name")
	command.MarkFlagRequired("volumename")
	command.MarkFlagRequired("imagedestination")

	if err := command.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
