FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    jq \
    openssh-client \
    tar \
    unzip \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS (for Claude Code)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Rename the default ubuntu user (UID 1000) to coder
RUN usermod -l coder -d /home/coder -m ubuntu \
    && groupmod -n coder ubuntu
RUN mkdir -p /workspace && chown coder:coder /workspace

USER coder
WORKDIR /workspace

ENTRYPOINT ["sleep", "infinity"]
