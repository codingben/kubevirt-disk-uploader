# KubeVirt Disk Uploader

Extracts disk and uploads it to a container registry.

## About

A tool designed to automate the extraction of disks from KubeVirt Virtual Machines, package them into [Container Disks](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk), and upload them to the Container Registry.

## Workflow

KubeVirt Disk Uploader -> Download VM Disk -> Build New Container Disk -> Push To Container Registry

## Installation

**Prerequisites**

1. Ensure Virtual Machine (VM) is powered off. Data from VM can be exported only when it is not used.
2. Modify [kubevirt-disk-uploader](https://github.com/codingben/kubevirt-disk-uploader/blob/main/kubevirt-disk-uploader.yaml#L58) arguments (VM Name, New Container Disk Name, and Disk File).
3. Modify [kubevirt-disk-uploader-credentials](https://github.com/codingben/kubevirt-disk-uploader/blob/main/kubevirt-disk-uploader.yaml#L65-L74) of the external container registry (Username, Password and Hostname).

Deploy `kubevirt-disk-uploader` within the same namespace as the Virtual Machine (VM):

```
kubectl apply -f kubevirt-disk-uploader.yaml -n $VM_NAMESPACE
```

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
Extracts disk and uploads it to a container registry...
Applying VirutalMachineExport object to expose Virutal Machine data...
virtualmachineexport.export.kubevirt.io/example-vm created
Downloading disk image disk.img.gz from example-vm Virutal Machine...
waiting for VM Export example-vm status to be ready...
Downloading file: 472.09 MiB [==>________________] 5.13 MiB
...
Donwload completed successfully.
Converting raw disk image to qcow2 format...
20969472+0 records in
20969472+0 records out
10736369664 bytes (11 GB, 10 GiB) copied, 45.9953 s, 233 MB/s
./tmp/disk.qcow2
Building and uploading new container image with exported disk image...
2023/11/28 16:55:37 Image built successfully
2023/11/28 17:12:41 Image pushed successfully
Succesfully extracted disk image and uploaded it in a new container image to container registry.
```

5. Run the new container disk in a new Virtual Machine:

```
kubectl apply -f examples/example-vm-exported.yaml
```

## KubeVirt Documentation

Read more about the used API at [KubeVirt Export API](https://kubevirt.io/user-guide/operations/export_api).
