#!/bin/bash

vm_name=$1

function apply_vmexport() {
  echo "Applying VirutalMachineExport object to expose Virutal Machine data..."

	cat << END | kubectl apply -f -
apiVersion: export.kubevirt.io/v1alpha1
kind: VirtualMachineExport
metadata:
  name: $vm_name-vmexport
spec:
  source:
    apiGroup: "kubevirt.io"
    kind: VirtualMachine
    name: $vm_name
END
}

function download_disk_img() {
  echo "Downloading disk image from $vm_name virutal machine..."

  usr/bin/virtctl vmexport download $vm_name-vmexport --vm=$vm_name --output=tmp/disk.img
  ls -l tmp/disk.img
}

function build_disk_img() {
  echo "Building exported disk image in a new $vm_name-disk container image..."

  cat << END > tmp/Dockerfile
FROM scratch
ADD --chown=107:107 ./disk.img /disk/
END
  buildah build -t $vm_name-disk:latest ./tmp
}

function push_disk_img() {
  echo "Pushing the new container image to Quay registry..."

  buildah login --username ${QUAY_USERNAME} --password ${QUAY_PASSWORD} ${QUAY_URL}
  buildah tag $vm_name-disk:latest quay.io/boukhano/$vm_name-disk:latest
  buildah push quay.io/boukhano/$vm_name-disk:latest
}

apply_vmexport
download_disk_img
build_disk_img
push_disk_img

echo "Succesfully extracted disk image and uploaded it in a new container image to Quay registry."
