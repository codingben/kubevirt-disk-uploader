package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kvcorev1 "kubevirt.io/api/core/v1"
	v1beta1 "kubevirt.io/api/export/v1beta1"
	kubecli "kubevirt.io/client-go/kubecli"
	tar "kubevirt.io/containerdisks/pkg/build"
)

const (
	pollInterval               = 15 * time.Second
	pollTimeout                = 3600 * time.Second
	diskPath            string = "./tmp/disk.qcow2"
	certificatePath     string = "./tmp/tls.crt"
	kvExportTokenKey    string = "token"
	kvExportTokenHeader string = "x-kubevirt-export-token"
	kvExportTokenLength int    = 20
)

func createSecret(client kubecli.KubevirtClient, vmNamespace, vmName string) error {
	token, err := generateSecureRandomString(kvExportTokenLength)
	if err != nil {
		return err
	}

	v1Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmName,
			Namespace: vmNamespace,
		},
		StringData: map[string]string{
			kvExportTokenKey: token,
		},
	}

	if err := setPodOwnerReference(client, v1Secret); err != nil {
		return err
	}

	_, err = client.CoreV1().Secrets(vmNamespace).Create(context.Background(), v1Secret, metav1.CreateOptions{})
	return err
}

func createVirtualMachineExport(client kubecli.KubevirtClient, vmNamespace, vmName string) error {
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

	if err := setPodOwnerReference(client, v1VmExport); err != nil {
		return err
	}

	_, err := client.VirtualMachineExport(vmNamespace).Create(context.Background(), v1VmExport, metav1.CreateOptions{})
	return err
}

func getRawDiskUrlFromVolumes(client kubecli.KubevirtClient, vmNamespace, vmName, volumeName string) (string, error) {
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

func waitUntilVirtualMachineExportReady(client kubecli.KubevirtClient, vmNamespace, vmName string) error {
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

func getCertificateFromVirtualMachineExport(client kubecli.KubevirtClient, vmNamespace, vmName string) (string, error) {
	vmExport, err := client.VirtualMachineExport(vmNamespace).Get(context.Background(), vmName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if vmExport.Status.Links == nil && vmExport.Status.Links.Internal == nil {
		return "", fmt.Errorf("no links found in VirtualMachineExport status")
	}

	content := vmExport.Status.Links.Internal.Cert
	if content == "" {
		return "", fmt.Errorf("no certificate found in VirtualMachineExport status")
	}
	return content, nil
}

func getExportToken(client kubecli.KubevirtClient, vmNamespace, vmName string) (string, error) {
	secret, err := client.CoreV1().Secrets(vmNamespace).Get(context.Background(), vmName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	data := secret.Data[kvExportTokenKey]
	if len(data) == 0 {
		return "", fmt.Errorf("failed to get export token from '%s/%s'", vmNamespace, vmName)
	}
	return string(data), nil
}

func downloadDiskImageFromURL(rawDiskUrl, exportToken string) error {
	cmd := exec.Command(
		"nbdkit",
		"-r",
		"curl",
		rawDiskUrl,
		fmt.Sprintf("header=%s: %s", kvExportTokenHeader, exportToken),
		fmt.Sprintf("cainfo=%s", certificatePath),
		"--run",
		fmt.Sprintf("qemu-img convert \"$uri\" -O qcow2 %s", diskPath),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	if fileInfo, err := os.Stat(diskPath); err != nil || fileInfo.Size() == 0 {
		return fmt.Errorf("disk image file does not exist or is empty")
	}
	return nil
}

func buildContainerDisk() (v1.Image, error) {
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
	return image, nil
}

func pushContainerDisk(image v1.Image, imageDestination string, pushTimeout int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(pushTimeout))
	defer cancel()

	auth := &authn.Basic{
		Username: os.Getenv("ACCESS_KEY_ID"),
		Password: os.Getenv("SECRET_KEY"),
	}
	err := crane.Push(image, imageDestination, crane.WithAuth(auth), crane.WithContext(ctx))
	if err != nil {
		log.Fatalf("Error pushing image: %v", err)
		return err
	}
	return nil
}

func run(client kubecli.KubevirtClient, vmNamespace, vmName, volumeName, imageDestination string, pushTimeout int) error {
	log.Printf("Creating a new Secret '%s/%s' object...", vmNamespace, vmName)

	if err := createSecret(client, vmNamespace, vmName); err != nil {
		return err
	}

	log.Printf("Creating a new VirtualMachineExport '%s/%s' object...", vmNamespace, vmName)

	if err := createVirtualMachineExport(client, vmNamespace, vmName); err != nil {
		return err
	}

	log.Println("Waiting for VirtualMachineExport status to be ready...")

	if err := waitUntilVirtualMachineExportReady(client, vmNamespace, vmName); err != nil {
		return err
	}

	log.Println("Getting raw disk URL from the VirtualMachineExport object status...")

	rawDiskUrl, err := getRawDiskUrlFromVolumes(client, vmNamespace, vmName, volumeName)
	if err != nil {
		return err
	}

	log.Println("Creating TLS certificate file from the VirtualMachineExport object status...")

	certificate, err := getCertificateFromVirtualMachineExport(client, vmNamespace, vmName)
	if err != nil {
		return err
	}

	if err := createFile(certificate); err != nil {
		return err
	}

	log.Println("Getting export token from the Secret object...")

	exportToken, err := getExportToken(client, vmNamespace, vmName)
	if err != nil {
		return err
	}

	log.Println("Downloading disk image from the VirtualMachineExport server...")

	if err := downloadDiskImageFromURL(rawDiskUrl, exportToken); err != nil {
		return err
	}

	log.Println("Building a new container image...")

	image, err := buildContainerDisk()
	if err != nil {
		return err
	}

	log.Println("Pushing new container image to the container registry...")

	if err := pushContainerDisk(image, imageDestination, pushTimeout); err != nil {
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

			namespace := getNamespace(vmNamespace)

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

func getNamespace(vmNamespace string) string {
	namespace := os.Getenv("VM_NAMESPACE")
	if namespace != "" {
		return namespace
	}

	return vmNamespace
}

func createFile(data string) error {
	file, err := os.OpenFile(certificatePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(data)
	if err != nil {
		return fmt.Errorf("failed to write content to file: %w", err)
	}
	return nil
}

func generateSecureRandomString(n int) (string, error) {
	// Alphanums is the list of alphanumeric characters used to create a securely generated random string
	Alphanums := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	ret := make([]byte, n)
	for i := range ret {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(Alphanums))))
		if err != nil {
			return "", err
		}
		ret[i] = Alphanums[num.Int64()]
	}

	return string(ret), nil
}

func getTaskRunPod(client kubecli.KubevirtClient) (*corev1.Pod, error) {
	podName, isSet := os.LookupEnv("VM_NAME")
	if !isSet {
		return nil, fmt.Errorf("pod name env variable is not set")
	}

	podNamespace, isSet := os.LookupEnv("VM_NAMESPACE")
	if !isSet {
		return nil, fmt.Errorf("pod namespace env variable is not set")
	}

	pod := &corev1.Pod{}
	pod, err := client.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	return pod, err
}

func setPodOwnerReference(client kubecli.KubevirtClient, object metav1.Object) error {
	pod, err := getTaskRunPod(client)
	if err != nil {
		return err
	}

	if object.GetNamespace() != pod.GetNamespace() {
		return fmt.Errorf("can't create owner reference for objects in different namespaces")
	}

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	gvks, _, err := scheme.ObjectKinds(pod)
	if err != nil {
		return fmt.Errorf("could not get GroupVersionKind for object: %w", err)
	}
	ref := metav1.OwnerReference{
		APIVersion: gvks[0].GroupVersion().String(),
		Kind:       gvks[0].Kind,
		UID:        pod.GetUID(),
		Name:       pod.GetName(),
	}

	object.SetOwnerReferences([]metav1.OwnerReference{ref})
	return nil
}
