[Return to OSSM Docs](../)

# Installing the istioctl tool

The `istioctl` tool is a configuration command line utility that allows service 
operators to debug and diagnose Istio service mesh deployments.

## Prerequisites

* Use an `istioctl` version that is the same version as the Istio control plane 
for the Service Mesh deployment. For checking the `Istio` version, run the following command:

    ```bash
    $ kubectl -n istio-system get istio
    ```

## Steps

1. Download `istioctl` binary

    In the OpenShift console, navigate to the Command Line Tools by clicking :grey_question: -> **Command Line Tools** in the upper-right of the header.  
    Then click on **Download istioctl** and choose the right architecture according to your system.

    **NB**: All the releases of `istioctl` are directly downloadable [here](https://mirror.openshift.com/pub/cgw/servicemesh/)

1. Extract the `istioctl` binary and add the client to your path, on your system.

    ```bash
    $ tar xzf istioctl-<OS>-<ARCH>.tar.gz -C $HOME/.istioctl/bin
    $ export PATH=$HOME/.istioctl/bin:$PATH
    ```

1. Confirm that the `istioctl` client version and the Istio control plane 
version now match (or are within one version) by running the following command
at the terminal:
  
    ```sh
    $ istioctl version
    ```

## Supported commands

|Command                | Description                                                                            | Supported          |
|-----------------------|----------------------------------------------------------------------------------------|--------------------|
| admin                 | Manage control plane (istiod) configuration                                            | :white_check_mark: |
| analyze               | Analyze Istio configuration and print validation messages                              | :white_check_mark: |
| authz                 | (authz is experimental. Use `istioctl experimental authz`)                             |                    |
| bug-report            | Cluster information and log capture support tool.                                      | :white_check_mark: |
| completion            | Generate the autocompletion script for the specified shell                             |                    |
| create-remote-secret  | Create a secret with credentials to allow Istio to access remote Kubernetes apiservers | :white_check_mark: |
| dashboard             | Access to Istio web UIs                                                                |                    |
| experimental          | Experimental commands that may be modified or deprecated                               |                    |
| help                  | Help about any command                                                                 | :white_check_mark: |
| install               | Applies an Istio manifest, installing or reconfiguring Istio on a cluster.             | :x:                |
| kube-inject           | Inject Istio sidecar into Kubernetes pod resources                                     | :x:                |
| manifest              | Commands related to Istio manifests                                                    |                    |
| operator              | Commands related to Istio operator controller.                                         | :x:                |
| profile               | Commands related to Istio configuration profiles                                       | :x:                |
| proxy-config          | Retrieve information about proxy configuration from Envoy [kube only]                  | :white_check_mark: |
| proxy-status          | Retrieves the synchronization status of each Envoy in the mesh                         | :white_check_mark: |
| remote-clusters       | Lists the remote clusters each istiod instance is connected to.                        |                    |
| tag                   | Command group used to interact with revision tags                                      |                    |
| uninstall             | Uninstall Istio from a cluster                                                         | :x:                |
| upgrade               | Upgrade Istio control plane in-place                                                   | :x:                |
| validate              | Validate Istio policy and rules files                                                  |                    |
| verify-install        | Verifies Istio Installation Status                                                     | :x:                |
| version               | Prints out build version information                                                   | :white_check_mark: |
| waypoint              | Manage waypoint configuration                                                          |                    |
| ztunnel-config        | Update or retrieve current Ztunnel configuration.                                      |                    |

