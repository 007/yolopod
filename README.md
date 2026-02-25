# YOLOpod - easy sandboxing for Claude Code

YOLOpod gives Claude Code a disposable sandbox so developers can run it in full-autonomy mode without risk to their local system. It spins up an isolated container with just the code, credentials, and tools needed for a session, then syncs changes back and cleans up when done. It runs on a local machine with KinD or on a shared Kubernetes cluster where many developers can work simultaneously.

