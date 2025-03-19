FROM kindest/node:latest

ARG CRIO_VERSION
ARG PROJECT_PATH=prerelease:/$CRIO_VERSION

# Install dependencies for CRIU.
RUN apt-get update -y && apt-get install -y \
    build-essential \
    libprotobuf-dev \
    libprotobuf-c-dev \
    protobuf-c-compiler \
    protobuf-compiler \
    python3-protobuf \
    libnl-3-dev \
    libcap-dev \
    libnet-dev \
    pkg-config \
    git \
    wget \
    curl \
    software-properties-common \
    vim \
    gnupg \
    uuid-dev \
    libbsd-dev \
    libdrm-dev \
    gnutls-dev \
    libnftables-dev

# Install CRIU from source so we can use the latest version compatible with the Linux kernel.
RUN cd /tmp && \
    git clone https://github.com/checkpoint-restore/criu.git && \
    cd criu && \
    make && \
    mv criu/criu /usr/bin/criu

# Install cri-o from source using the given version.
RUN echo "Installing Packages ..." \
    && apt-get clean \
    && apt-get update -y \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    software-properties-common vim gnupg \
    && echo "Installing cri-o ..." \
    && curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/$PROJECT_PATH/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/$PROJECT_PATH/deb/ /" | tee /etc/apt/sources.list.d/cri-o.list \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get --option=Dpkg::Options::=--force-confdef install -y cri-o \
    && sed -i 's/containerd/crio/g' /etc/crictl.yaml

# Configure cri-o to use CRIU for checkpoint/restore.
COPY crio.conf /etc/crio/crio.conf

# Configuration so CRIU can checkpoint Pods in the cluster.
COPY criu.conf /etc/criu/default.conf

# Disable containerd and enable cri-o.
RUN systemctl disable containerd && systemctl enable crio
