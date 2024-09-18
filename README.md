# Homelab GitOps: An ArgoCD-Powered Kubernetes Cluster

[![GitOps](https://img.shields.io/badge/GitOps-ArgoCD-ef9421?style=for-the-badge&logo=argo)](https://argo-cd.readthedocs.io/en/stable/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-kubeadm-326ce5?style=for-the-badge&logo=kubernetes)](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg?style=for-the-badge)](https://www.gnu.org/licenses/gpl-3.0)

Welcome to my Homelab GitOps repository! This repository contains the entire configuration for a Kubernetes cluster running in my homelab. The cluster is managed by ArgoCD, ensuring that the entire state of the cluster is described declaratively in this Git repository.

## Overview

The primary goal of this project is to maintain a powerful, flexible, and fully automated homelab environment. By leveraging GitOps, every change to the cluster is version-controlled, auditable, and easily reversible.

This setup is designed to be:

-   **Declarative:** The desired state of the entire cluster, from infrastructure to applications, is defined in code.
-   **Version-Controlled:** Git serves as the single source of truth. Every change is a commit.
-   **Automated:** ArgoCD continuously monitors the repository and applies any changes to the cluster automatically.
-   **Reproducible:** The entire cluster setup can be recreated from the contents of this repository.

## Architecture

### Hardware and OS

-   **Compute:** The cluster runs on a set of Intel NUCs, which provide a great balance of performance and low power consumption.
-   **Boot Process:** The NUCs are diskless and boot over the network using **iPXE**. This allows for centralized management of the operating system images and ensures that all nodes are running the exact same OS version.

### Kubernetes

-   **Distribution:** The cluster is a vanilla Kubernetes installation provisioned with `kubeadm`. `kubeadm` is a simple and effective tool for creating production-ready Kubernetes clusters.
-   **CNI:** [Flannel](https://github.com/flannel-io/flannel) is used as the CNI plugin to provide networking between the pods.
-   **Ingress:** [Istio](https://istio.io/) is used as an ingress gateway and for managing service-to-service communication within the cluster.
-   **Storage:** Storage is managed by a variety of solutions, likely including operators for block or object storage, provisioned as needed by applications.

### GitOps and Automation

-   **ArgoCD:** The core of the GitOps workflow. ArgoCD is installed on the cluster and configured to watch this repository.
-   **App of Apps Pattern:** We use an `ApplicationSet` to implement a variation of the "App of Apps" pattern. A single `ApplicationSet` in `argocd-bootstrap/applicationset.yaml` discovers and deploys all the applications defined in the `apps/` directory.
-   **Secrets Management:** Secrets are encrypted using [Mozilla SOPS](https://github.com/mozilla/sops) and committed to the repository. The `external-secrets` operator, combined with a secret store (like Vault, GCP/AWS Secrets Manager, etc.), decrypts them securely within the cluster.
-   **Dependency Management:** [Renovate](https://www.mend.io/free-developer-tools/renovate/) is used to automatically create pull requests for dependency updates (e.g., new Helm chart versions, container image tags), ensuring the services are always up-to-date.

## Repository Structure

The repository is organized to support the GitOps workflow:

```plaintext
.
├── apps/
│   ├── 00-flannel/
│   ├── 01-certificates/
│   ├── ... (many more applications)
├── argocd-bootstrap/
│   ├── applicationset.yaml  # The root ApplicationSet that deploys all apps
│   ├── argocd/              # ArgoCD's own configuration
│   ├── ...
│   └── patches/             # Kustomize patches for ArgoCD
├── LICENSE
├── README.md
└── renovate.json5           # Renovate configuration
```

-   **`apps/`**: This directory contains a subdirectory for each application deployed in the cluster. The numerical prefixes enforce an ordering for deployment (e.g., infrastructure services first). Each subdirectory is a self-contained ArgoCD Application source, typically containing a `kustomization.yaml` or a Helm chart definition.
-   **`argocd-bootstrap/`**: This is the starting point for bootstrapping the cluster. It contains the `ApplicationSet` that tells ArgoCD to manage all the applications in the `apps/` directory.

## Getting Started

To bootstrap a new cluster using this repository, you would follow these general steps:

1.  **Provision Hardware and Network:**
    -   Set up the Intel NUCs.
    -   Configure a DHCP and TFTP server for iPXE booting.
    -   Serve the desired Linux distribution for the Kubernetes nodes.

2.  **Create the Kubernetes Cluster:**
    -   Use `kubeadm` to initialize the control plane on the first node.
    -   Join the other nodes to the cluster as workers.

3.  **Install ArgoCD:**
    -   Install the ArgoCD CLI and operator on the cluster. Refer to the [official ArgoCD documentation](https://argo-cd.readthedocs.io/en/stable/getting_started/).

4.  **Bootstrap ArgoCD:**
    -   Apply the `ApplicationSet` to the cluster. This will configure ArgoCD to manage itself and all other applications.
        ```bash
        kubectl apply -f argocd-bootstrap/applicationset.yaml
        ```
    -   ArgoCD will then see the `ApplicationSet` and start deploying all the applications defined in the `apps/` directory.

## Managing Applications

### Adding a New Application

To add a new application to the cluster:

1.  Create a new subdirectory in the `apps/` directory (e.g., `apps/my-new-app/`).
2.  Add the Kubernetes manifests, Kustomization file, or Helm chart for the new application in that subdirectory.
3.  Commit and push the changes to this Git repository.

ArgoCD's `ApplicationSet` will automatically detect the new directory and deploy the application to the cluster.

### Updating an Application

To update an application, simply modify its manifests in the corresponding subdirectory in the `apps/` directory and push the changes. ArgoCD will detect the change and automatically apply the update to the cluster.

### Secrets Management Workflow

This repository uses SOPS for encrypting secrets.

1.  **To encrypt a secret:**
    ```bash
    sops --encrypt --in-place secret.yaml
    ```
2.  **To edit an encrypted secret:**
    ```bash
    sops secret.yaml
    ```
    This will open the decrypted file in your default editor. When you save and close the editor, SOPS will automatically re-encrypt the file.

## Contributing

While this is a personal homelab repository, contributions, suggestions, and feedback are welcome. If you have any ideas for improvements, feel free to open an issue or a pull request.

## License

This project is licensed under the **GNU General Public License v3.0**. See the [LICENSE](LICENSE) file for details.
