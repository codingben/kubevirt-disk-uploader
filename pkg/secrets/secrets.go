package secrets

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/codingben/kubevirt-disk-uploader/pkg/ownerreference"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubecli "kubevirt.io/client-go/kubecli"
)

func CreateVirtualMachineExportSecret(client kubecli.KubevirtClient, namespace, name string) error {
	length := 20
	token, err := GenerateSecureRandomString(length)
	if err != nil {
		return err
	}

	v1Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"token": token,
		},
	}

	if err := ownerreference.SetPodOwnerReference(client, v1Secret); err != nil {
		return err
	}

	_, err = client.CoreV1().Secrets(namespace).Create(context.Background(), v1Secret, metav1.CreateOptions{})
	return err
}

func GetTokenFromVirtualMachineExportSecret(client kubecli.KubevirtClient, namespace, name string) (string, error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	data := secret.Data["token"]
	if len(data) == 0 {
		return "", fmt.Errorf("failed to get export token from '%s/%s'", namespace, name)
	}
	return string(data), nil
}

func GenerateSecureRandomString(n int) (string, error) {
	// Alphanums is the list of alphanumeric characters used to create a securely generated random string
	alphanums := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	ret := make([]byte, n)
	for i := range ret {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphanums))))
		if err != nil {
			return "", err
		}
		ret[i] = alphanums[num.Int64()]
	}

	return string(ret), nil
}
