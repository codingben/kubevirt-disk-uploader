FROM golang:1.22 as builder

WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GODEBUG=http2client=0 GOARCH=amd64 go build -o kubevirt-disk-uploader .

FROM quay.io/fedora/fedora-minimal:39

RUN cd /usr/bin && \
    curl -L https://github.com/kubevirt/kubevirt/releases/download/v1.0.0/virtctl-v1.0.0-linux-amd64 --output virtctl && \
    chmod +x virtctl && \
    microdnf install -y gzip qemu-img libguestfs-tools-c && \
    microdnf clean all -y
COPY --from=builder /app/kubevirt-disk-uploader /usr/local/bin/kubevirt-disk-uploader

ENTRYPOINT ["/usr/local/bin/kubevirt-disk-uploader"]
