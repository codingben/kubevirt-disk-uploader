#!/bin/sh

# Arguments
VM_NAME=$1
CONTAINER_DISK_NAME=$2
DISK_FILE=$3

# Variables
OUTPUT_PATH=./tmp
TEMP_DISK_PATH=$OUTPUT_PATH/$DISK_FILE

validate_arguments() {
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

apply_vmexport() {
  echo "Applying VirutalMachineExport object to expose Virutal Machine data..."

  cat <<END | kubectl apply -f -
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

download_disk_img() {
  echo "Downloading disk image $DISK_FILE from $VM_NAME Virutal Machine..."

  usr/bin/virtctl vmexport download "$VM_NAME" --vm="$VM_NAME" --output="$TEMP_DISK_PATH"

  if [ -e "$TEMP_DISK_PATH" ] && [ -s "$TEMP_DISK_PATH" ]; then
    echo "Donwload completed successfully."
  else
    echo "Download failed."
    exit 1
  fi
}

convert_disk_img() {
  echo "Converting raw disk image to qcow2 format..."

  CONVERTED_DISK_PATH=$OUTPUT_PATH/disk.qcow2

  zcat "$TEMP_DISK_PATH" | dd conv=sparse of="${TEMP_DISK_PATH%.gz}"
  qemu-img convert -f raw -O qcow2 "${TEMP_DISK_PATH%.gz}" $CONVERTED_DISK_PATH

  if [ ! -e $CONVERTED_DISK_PATH ] || [ ! -s $CONVERTED_DISK_PATH ]; then
    echo "Downloaded disk was not converted into qcow2 format."
    exit 1
  fi
}

upload_container_img() {
  echo "Building and uploading new container image with exported disk image..."

  DISK_PATH=$OUTPUT_PATH/disk.qcow2
  IMAGE_DESTINATION=$REGISTRY_HOST/$REGISTRY_USERNAME/$CONTAINER_DISK_NAME

  kubevirt-disk-uploader -d $DISK_PATH -i "$IMAGE_DESTINATION"
}

main() {
  echo "Extracts disk and uploads it to a container registry..."

  validate_arguments
  apply_vmexport
  download_disk_img
  convert_disk_img
  upload_container_img

  echo "Succesfully extracted disk image and uploaded it in a new container image to container registry."
}

main
