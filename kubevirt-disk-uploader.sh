#!/bin/bash

vm_name=$1
container_disk_name=$2

function apply_vmexport() {
  echo "Applying VirutalMachineExport object to expose Virutal Machine data..."

	cat << END | kubectl apply -f -
apiVersion: export.kubevirt.io/v1alpha1
kind: VirtualMachineExport
metadata:
  name: $vm_name
spec:
  source:
    apiGroup: "kubevirt.io"
    kind: VirtualMachine
    name: $vm_name
END
}

function download_disk_img() {
  echo "Downloading disk image from $vm_name virutal machine..."

  file="tmp/disk.img.gz"

  usr/bin/virtctl vmexport download $vm_name --vm=$vm_name --output=$file

  if [ -e "$file" ] && [ -s "$file" ]; then
      echo "Donwload completed successfully."
  else
      echo "Download failed."
      exit 1
  fi
}

function convert_disk_img() {
  echo "Converting disk image to qcow2..."

  gunzip tmp/disk.img.gz
  qemu-img convert -f raw -O qcow2 tmp/disk.img tmp/disk.qcow2
  rm tmp/disk.img
}

function build_disk_img() {
  echo "Building exported disk image in a new container image..."

  cat << END > tmp/Dockerfile
FROM scratch
ADD --chown=107:107 ./disk.qcow2 /disk/
END
  buildah build -t $container_disk_name ./tmp
}

function push_disk_img() {
  echo "Pushing the new container image to Quay registry..."

  buildah login --username ${REGISTRY_USERNAME} --password ${REGISTRY_PASSWORD} ${REGISTRY_HOST}
  buildah tag $container_disk_name ${REGISTRY_HOST}/${REGISTRY_USERNAME}/$container_disk_name
  buildah push ${REGISTRY_HOST}/${REGISTRY_USERNAME}/$container_disk_name
}

apply_vmexport
download_disk_img
convert_disk_img
build_disk_img
push_disk_img

echo "Succesfully extracted disk image and uploaded it in a new container image to Quay registry."
