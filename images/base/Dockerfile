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

# Create non-root user
RUN useradd -m -s /bin/bash -u 1000 coder
RUN mkdir -p /workspace && chown coder:coder /workspace

USER coder
WORKDIR /workspace

ENTRYPOINT ["sleep", "infinity"]
