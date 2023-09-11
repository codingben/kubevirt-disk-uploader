#!/bin/bash

vm_name=$1
container_disk_name=$2
disk_file=$3
disk_path=tmp/$3

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
  echo "Downloading disk image $disk_file from $vm_name Virutal Machine..."

  usr/bin/virtctl vmexport download $vm_name --vm=$vm_name --output=$disk_path

  if [ -e "$disk_path" ] && [ -s "$disk_path" ]; then
      echo "Donwload completed successfully."
  else
      echo "Download failed."
      exit 1
  fi
}

function convert_disk_img() {
  echo "Converting raw disk image to qcow2 format..."

  gunzip $disk_path
  qemu-img convert -f raw -O qcow2 $disk_path tmp/disk.qcow2
  rm $disk_path
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
  echo "Pushing the new container image to container registry..."

  buildah login --username ${REGISTRY_USERNAME} --password ${REGISTRY_PASSWORD} ${REGISTRY_HOST}
  buildah tag $container_disk_name ${REGISTRY_HOST}/${REGISTRY_USERNAME}/$container_disk_name
  buildah push ${REGISTRY_HOST}/${REGISTRY_USERNAME}/$container_disk_name
}

apply_vmexport
download_disk_img
convert_disk_img
build_disk_img
push_disk_img

echo "Succesfully extracted disk image and uploaded it in a new container image to container registry."
