FROM quay.io/fedora/fedora:38

RUN cd /usr/bin && \
	curl -L https://github.com/kubevirt/kubevirt/releases/download/v1.0.0/virtctl-v1.0.0-linux-amd64 --output virtctl && \
	chmod +x virtctl && \
	curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
	chmod +x kubectl
RUN curl -LO https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz && \
    tar -xzf openshift-client-linux.tar.gz -C /usr/local/bin/ && \
    rm -f openshift-client-linux.tar.gz

RUN dnf install -y buildah qemu-img && \
	dnf clean all -y

COPY kubevirt-disk-uploader.sh /usr/bin/kubevirt-disk-uploader.sh

ENTRYPOINT ["/usr/bin/kubevirt-disk-uploader.sh"]
