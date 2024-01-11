# KubeVirt Disk Uploader

Extracts disk and uploads it to a container registry.

## About

A tool designed to automate the extraction of disks from KubeVirt Virtual Machines, package them into [Container Disks](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk), and upload them to the Container Registry.

## Workflow

KubeVirt Disk Uploader -> Download VM Disk -> Build New Container Disk -> Push To Container Registry

## Installation

**Prerequisites**

1. Ensure Virtual Machine (VM) is powered off. Data from VM can be exported only when it is not used.
2. Modify [kubevirt-disk-uploader](https://github.com/codingben/kubevirt-disk-uploader/blob/main/kubevirt-disk-uploader.yaml#L58) arguments (VM Name, New Container Disk Name, Disk File, and Enable or Disable System Preparation).
3. Modify [kubevirt-disk-uploader-credentials](https://github.com/codingben/kubevirt-disk-uploader/blob/main/kubevirt-disk-uploader.yaml#L65-L74) of the external container registry (Username, Password and Hostname).

Deploy `kubevirt-disk-uploader` within the same namespace as the Virtual Machine (VM):

```
kubectl apply -f kubevirt-disk-uploader.yaml -n $VM_NAMESPACE
```

## KubeVirt Documentation

Read more about the used API at [KubeVirt Export API](https://kubevirt.io/user-guide/operations/export_api).
