# KubeVirt Disk Uploader

Extracts disk and uploads it to container registry.

## About

A tool designed to automate the extraction of disks from KubeVirt Virtual Machines, package them into Docker images, and upload them to the container registry.

## Workflow

KubeVirt Disk Uploader -> KubeVirt VM Export -> VM Export Proxy & Server -> Download VM Disk -> Build New Disk Container -> Push To Container Registry

## Installation

This pod below will create a new KubeVirt VM Export object which starts a new `virt-exportserver` container, by running `virtctl vmexport ...` to download the disk image from the desired Virtual Machine. Once downloaded, exported disk image will be copied to a new container image that is uploaded to the container registry.

```
kubectl apply -f kubevirt-disk-uploader.yaml
```

**Note**: A Virtual Machine can only be exported when it is powered off.

## Example

1. Enable VMExport:

```
kubectl apply -f examples/enable-vmexport.yaml
```

2. Create a new example Virtual Machine:

```
kubectl apply -f examples/example-vm.yaml
```

3. Create a new KubeVirt Disk Uploader:

```
kubectl apply -f kubevirt-disk-uploader.yaml
```

4. See the logs of KubeVirt Disk Uploader:

```
kubectl logs kubevirt-disk-uploader
Downloading disk image from example-vm virutal machine...        
Applying VirutalMachineExport object to expose Virutal Machine data...
virtualmachineexport.export.kubevirt.io/example-vm-vmexport created
waiting for VM Export example-vm-vmexport status to be ready...
Downloading file: 126.46 KiB [==>________________]
...
Download finished succesfully
VirtualMachineExport 'ben-dev/example-vm-vmexport' deleted succesfully
-rw-r--r--. 1 root root 730270529 Sep  6 14:34 tmp/disk.img
Building exported disk image in a new example-vm-disk container image...
STEP 1/2: FROM scratch
STEP 2/2: ADD --chown=107:107 ./disk.img /disk/
COMMIT boukhano/example-vm-disk:latest
Getting image source signatures
Copying ...
Writing manifest to image destination
--> ...
Successfully tagged localhost/boukhano/example-vm-disk:latest
Pushing the new container image to Quay registry...
Login Succeeded!
Getting image source signatures
Copying ...
Writing manifest to image destination
Succesfully extracted disk image and uploaded it in a new container image to Quay registry.
```

5. Run the new container disk in a new Virtual Machine:

```
kubectl apply -f examples/exported-vm.yaml
```

## Official Documentation

Read more at [KubeVirt Export API](https://kubevirt.io/user-guide/operations/export_api).
