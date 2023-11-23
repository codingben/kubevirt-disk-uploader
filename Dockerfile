FROM golang:1.21 as builder

WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kubevirt-disk-uploader .

FROM quay.io/fedora/fedora:38

RUN cd /usr/bin && \
    curl -L https://github.com/kubevirt/kubevirt/releases/download/v1.0.0/virtctl-v1.0.0-linux-amd64 --output virtctl && \
    chmod +x virtctl && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    dnf install -y qemu-img && \
    dnf clean all -y
COPY --from=builder /app/kubevirt-disk-uploader /usr/local/bin/kubevirt-disk-uploader
COPY run-uploader.sh /usr/local/bin/run-uploader.sh

ENTRYPOINT ["/usr/local/bin/run-uploader.sh"]
