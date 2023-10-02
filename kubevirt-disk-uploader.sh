#!/bin/sh

# Arguments
VM_NAME=$1
CONTAINER_DISK_NAME=$2
DISK_FILE=$3

# Variables
OUTPUT_PATH=./tmp
TEMP_DISK_PATH=$OUTPUT_PATH/$DISK_FILE

function validate_arguments() {
  if [ -z "$VM_NAME" ]; then
    echo "Virtual Machine name is missing. Please provide a valid VM name."
    exit 1
  fi

  if [ -z "$CONTAINER_DISK_NAME" ]; then
    echo "Container Disk name is missing. Please provide a valid disk name."
    exit 1
  fi

  if [ -z "$DISK_FILE" ]; then
    echo "Disk file to extract is missing. Please provide a valid disk file."
    exit 1
  fi
}

function apply_vmexport() {
  echo "Applying VirutalMachineExport object to expose Virutal Machine data..."

	cat << END | kubectl apply -f -
apiVersion: export.kubevirt.io/v1alpha1
kind: VirtualMachineExport
metadata:
  name: $VM_NAME
spec:
  source:
    apiGroup: "kubevirt.io"
    kind: VirtualMachine
    name: $VM_NAME
END
}

function download_disk_img() {
  echo "Downloading disk image $DISK_FILE from $VM_NAME Virutal Machine..."

  usr/bin/virtctl vmexport download $VM_NAME --vm=$VM_NAME --output=$TEMP_DISK_PATH

  if [ -e "$TEMP_DISK_PATH" ] && [ -s "$TEMP_DISK_PATH" ]; then
      echo "Donwload completed successfully."
  else
      echo "Download failed."
      exit 1
  fi
}

function convert_disk_img() {
  echo "Converting raw disk image to qcow2 format..."

  CONVERTED_DISK_PATH=$OUTPUT_PATH/disk.qcow2

  zcat $TEMP_DISK_PATH | dd conv=sparse of=${TEMP_DISK_PATH%.gz}
  qemu-img convert -f raw -O qcow2 ${TEMP_DISK_PATH%.gz} $CONVERTED_DISK_PATH

  if [ ! -e $CONVERTED_DISK_PATH ] || [ ! -s $CONVERTED_DISK_PATH ]; then
    echo "Downloaded disk was not converted into qcow2 format."
    exit 1
  fi
}

function build_container_img() {
  echo "Building new container image with exported disk image..."

  DOCKERFILE_PATH=$OUTPUT_PATH/Dockerfile

  cat << END > $DOCKERFILE_PATH
FROM scratch
ADD --chown=107:107 ./disk.qcow2 /disk/
END
  buildah build -t $CONTAINER_DISK_NAME $OUTPUT_PATH
}

function check_container_img() {
  echo "Checking container image size..."
  
  IMAGE_SIZE=$(buildah images --format '{{.Size}}' --noheading $CONTAINER_DISK_NAME)

  echo "Container image size is ${IMAGE_SIZE}."
}

function push_container_img() {
  echo "Pushing the new container image to container registry..."

  REGISTRY_URL=${REGISTRY_HOST}/${REGISTRY_USERNAME}/$CONTAINER_DISK_NAME

  buildah login --username ${REGISTRY_USERNAME} --password ${REGISTRY_PASSWORD} ${REGISTRY_HOST}
  buildah tag $CONTAINER_DISK_NAME $REGISTRY_URL
  buildah push $REGISTRY_URL
}

function main() {
  echo "Extracts disk and uploads it to a container registry..."

  validate_arguments
  apply_vmexport
  download_disk_img
  convert_disk_img
  build_container_img
  check_container_img
  push_container_img

  echo "Succesfully extracted disk image and uploaded it in a new container image to container registry."
}

main
