# KubeVirt Disk Uploader Task

Automates the extraction of disk and uploads it to a container registry, to be used in multiple Kubernetes clusters.

# Example Scenario

When user runs [KubeVirt Tekton Tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) example pipelines (windows-installer, windows-customize) to prepare Windows disk images - The newly created disk image is only in a single cluster. If user wants to have it in another cluster, then KubeVirt Disk Uploader can be used to push it out of the cluster.

# Installation

```
kubectl apply -f https://raw.githubusercontent.com/codingben/kubevirt-disk-uploader/0.5.0/task/kubevirt-disk-uploader-task.yaml
```

# Parameters

- **VM_NAME**: The name of the virtual machine
- **VOLUME_NAME**: The volume name of the virtual machine
- **IMAGE_DESTINATION**: Destination of the image in container registry
- **ENABLE_VIRT_SYSPREP**: Enable or disable preparation of disk image
- **PUSH_TIMEOUT**: ContainerDisk push timeout in minutes

# Usage

Please apply the Task (with a Secret) and then apply TaskRun to run it.

Secret:

```
---
apiVersion: v1
kind: Secret
metadata:
  name: kubevirt-disk-uploader-credentials-tekton
type: Opaque
data:
  registryUsername: "<REGISTRY_USERNAME>"
  registryPassword: "<REGISTRY_PASSWORD>"
```

TaskRun:

```
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: kubevirt-disk-uploader-task-run
spec:
  taskRef:
    name: kubevirt-disk-uploader-task
  params:
  - name: VM_NAME
    value: <VM_NAME_VALUE>
  - name: VOLUME_NAME
    value: <VOLUME_NAME_VALUE>
  - name: IMAGE_DESTINATION
    value: <IMAGE_DESTINATION_VALUE>
  - name: ENABLE_VIRT_SYSPREP
    value: <ENABLE_VIRT_SYSPREP_VALUE>
  - name: PUSH_TIMEOUT
    value: <PUSH_TIMEOUT>
```
