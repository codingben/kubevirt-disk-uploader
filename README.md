# KubeVirt Disk Uploader

Extracts disk and uploads it to a container registry.

## About

A tool designed to automate the extraction of disks from KubeVirt Virtual Machines, package them into [Container Disks](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk), and upload them to the Container Registry.

## Workflow

KubeVirt Disk Uploader -> Download VM Disk -> Build New Container Disk -> Push To Container Registry

## Installation

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
Applying VirutalMachineExport object to expose Virutal Machine data...
virtualmachineexport.export.kubevirt.io/example-vm-vmexport created
Downloading disk image from example-vm virutal machine...
waiting for VM Export example-vm-vmexport status to be ready...
Downloading file: 126.46 KiB [==>________________]
...
VirtualMachineExport 'ben-dev/example-vm-vmexport' deleted succesfully
-rw-r--r--. 1 root root 730270529 Sep  7 12:57 tmp/disk.img.gz
Converting disk image to qcow2...
Building exported disk image in a new example-vm-disk container image...
STEP 1/2: FROM scratch
STEP 2/2: ADD --chown=107:107 ./disk.qcow2 /disk/
COMMIT example-vm-disk:latest
Getting image source signatures
Copying ..,
Writing manifest to image destination
Successfully tagged localhost/example-vm-disk:latest
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

## KubeVirt Documentation

Read more about the API at [KubeVirt Export API](https://kubevirt.io/user-guide/operations/export_api).
