# YOLOpod Technical Description

YOLOpod provisions ephemeral Kubernetes pods as isolated sandboxes for Claude Code sessions running in `--dangerously-skip-permissions` mode.

A developer invokes `yolopod` with a config file that specifies resource requests, credential files (mounted read-only), SSH agent forwarding, the Claude API key, and any additional tooling or setup scripts to layer onto a base image that ships with Claude Code and common dev tools.

An init container prepares the environment per the config, then the developer attaches to Claude Code as the primary interface inside the pod via `kubectl exec` or SSH.

When the session ends, a sidecar or preStop hook syncs git changes back to the developer's local workspace, after which the pod is destroyed with no persistent state.

YOLOpod runs locally via KinD for solo use, or against a shared remote cluster where API-heavy Claude sessions binpack efficiently across many developers on large machines.
