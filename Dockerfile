FROM golang:1.22 as builder

WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kubevirt-disk-uploader .

FROM quay.io/fedora/fedora-minimal:39

RUN microdnf install -y nbdkit qemu-img && microdnf clean all -y
COPY --from=builder /app/kubevirt-disk-uploader /usr/local/bin/kubevirt-disk-uploader

ENTRYPOINT ["/usr/local/bin/kubevirt-disk-uploader"]
