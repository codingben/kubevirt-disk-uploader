## Examples

### Manual Example

1. Enable VMExport in KubeVirt Custom Resource (CR):

```
kubectl apply -f examples/enable-vmexport.yaml
```

2. Deploy a new example Virtual Machine (VM):

```
kubectl apply -f examples/example-vm.yaml
```

3. Deploy KubeVirt Disk Uploader:

```
kubectl apply -f kubevirt-disk-uploader.yaml
```

4. Deploy exported Virtual Machine (VM):

```
kubectl apply -f examples/example-vm-exported.yaml
```

### Tekton Pipeline Example

Deploy example pipeline:

```
kubectl apply -f examples/kubevirt-disk-uploader-tekton.yaml
```

This pipeline will deploy a new Virutal Machine (VM), disk uploader, and then the exported Virutal Machine (VM).
