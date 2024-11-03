# KubeVirt Disk Uploader

Extracts disk and uploads it to a container registry.

**Note**: This repository is no longer maintained. Please refer to [kubevirt/kubevirt-tekton-tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) to use this functionality.

## About

A tool designed to automate the extraction of disk, rebuild as [Container Disk](https://kubevirt.io/user-guide/virtual_machines/disks_and_volumes/#containerdisk) and upload to the Container Registry.

## Usage

These are the supported export sources:

- VirtualMachine (VM)
- VirtualMachineSnapshot (VM Snapshot)
- PersistentVolumeClaim (PVC)

Data from the source can be exported only when it is not used.

**Prerequisites**

- Modify [kubevirt-disk-uploader](https://github.com/codingben/kubevirt-disk-uploader/blob/main/kubevirt-disk-uploader.yaml#L58) arguments.
- Modify [kubevirt-disk-uploader-credentials](https://github.com/codingben/kubevirt-disk-uploader/blob/main/kubevirt-disk-uploader.yaml#L65-L74) of the external container registry.

**Parameters**

- **Export Source Kind**: Specify the export source kind (`vm`, `vmsnapshot`, `pvc`).
- **Export Source Namespace**: The namespace of the export source.
- **Export Source Name**: The name of the export source.
- **Volume Name**:  The name of the volume to export data.
- **Image Destination**: Destination of the image in container registry (`$HOST/$OWNER/$REPO:$TAG`).
- **Push Timeout**: The push timeout of container disk to registry.

Deploy `kubevirt-disk-uploader` within the same namespace of Export Source (VM, VM Snapshot, PVC):

```
kubectl apply -f kubevirt-disk-uploader.yaml -n $POD_NAMESPACE
```

Setting of environment variable `POD_NAMESPACE` overrides the value in `--export-source-namespace` if passed.

## KubeVirt Documentation

Read more about the used API at [KubeVirt Export API](https://kubevirt.io/user-guide/operations/export_api).
