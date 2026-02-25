FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    jq \
    openssh-client \
    openssh-server \
    tar \
    unzip \
    wget \
    sudo \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /run/sshd

# Rename the default ubuntu user (UID 1000) to coder
RUN usermod -l coder -d /home/coder -m ubuntu \
    && groupmod -n coder ubuntu
RUN mkdir -p /workspace && chown coder:coder /workspace
RUN echo "coder ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/coder

USER coder
WORKDIR /workspace

ENV PATH="/home/coder/.local/bin:${PATH}"
RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> /home/coder/.profile

# Install Claude Code (native installer, no Node.js needed)
RUN curl -fsSL https://claude.ai/install.sh | bash

ENTRYPOINT ["sleep", "infinity"]
