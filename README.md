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

**Note**: A Virtual Machine can only be exported when it is powered off.

## Official Documentation

Read more at [KubeVirt Export API](https://kubevirt.io/user-guide/operations/export_api).
