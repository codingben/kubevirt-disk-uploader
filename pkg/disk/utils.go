package disk

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
)

func DownloadDiskImageFromURL(rawDiskUrl, headerKey, headerValue, certificatePath, diskPath string) error {
	cmd := exec.Command(
		"nbdkit",
		"-r",
		"curl",
		rawDiskUrl,
		fmt.Sprintf("header=%s: %s", headerKey, headerValue),
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

func CreateFile(path, data string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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
