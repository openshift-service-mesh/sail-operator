# Running Istio Integration Tests With Sail Operator

As the OpenShift Service Mesh (OSSM) evolves, it's critical to validate that Istio continues to function correctly when deployed using the Sail Operator, which is the default control plane installer for OSSM 3.x. However, upstream Istio integration test framework uses helm to perform Istio deployment. This misalignment creates challenges in testing OSSM features and regressions within the upstream test suites.

To bridge this gap, the integration test framework in upstream Istio has been extended to support running tests with Sail Operator. This allows developers and QE teams to confidently run comprehensive upstream test coverage in a Sail managed environment either on OpenShift cluster using the Sail Operator ensuring upstream compatibility, product quality, and future OSSM releases stability.

**Note:** Currently, the Integration test runner script [integ-suite-ocp.sh](https://github.com/openshift-service-mesh/istio/blob/master/prow/integ-suite-ocp.sh) from the downstream repository is going to be used to execute these tests.

### Prerequisites:
- To execute integration tests you need to locally clone the [service-mesh-istio](https://github.com/openshift-service-mesh/istio) project from github.
- OpenShift Service Mesh Operator 3.1 or later

## Getting Environment Ready
Before running the converter with the Istio testing framework, you must configure your environment.
You have two options for setting up the sail-operator:

- Use an already installed OSSM

- Deploy the Sail Operator from source during test execution

Each approach requires a different set of environment variables, as explained below.

- ### Using Preinstalled OSSM
If you already have OSSM (OpenShift Service Mesh) preinstalled, set the following environment variables:
```sh
CONTROL_PLANE_SOURCE=sail
TEST_HUB=docker.io/istio
TAG=1.26.2 #Modify for desired version
ISTIO_VERSION=v1.26.2 #Modify for desired version
```

- ### Deploying Sail Operator from Source
If you want the test runner to install the Sail operator from source, set:
```sh
CONTROL_PLANE_SOURCE=sail
INSTALL_SAIL_OPERATOR=true
```
This instructs the test runner script to deploy the Sail operator first and execute tests with supported control plane version.

### Optional Environment Variables (for deployment from source)
You may also define the following optional variables to control which Istio version is installed via the Sail operator:

- CONVERTER_BRANCH:
Specifies a Sail operator release branch. The converter will fetch the latest supported Istio version from:
```sh
https://raw.githubusercontent.com/istio-ecosystem/sail-operator/$CONVERTER_BRANCH/pkg/istioversion/versions.yaml
```
**Note:** Defaults to main

Example:
```sh
CONVERTER_BRANCH=release-3.1
```

## Executing Tests:
### Command to Execute

- To run pilot test suite execute:
```sh
prow/integ-suite-ocp.sh pilot 'TestCNIVersionSkew|TestGateway|TestAuthZCheck|TestKubeInject|TestRevisionTags|TestUninstallByRevision|TestUninstallWithSetFlag|TestUninstallCustomFile|TestUninstallPurge|TestCNIRaceRepair|TestValidation|TestWebhook|TestMultiRevision'
```
    Note: As you can see there are some skips that are not working yet over Openshift, this is managed under the Jira ticket https://issues.redhat.com/browse/OSSM-9328 and this documentation is going to be updated as soon the Jira is solved. 

- To run telemetry suite execute:
```sh
prow/integ-suite-ocp.sh telemetry
```

### Debugging the converter with the script logs:
Every execution of the converter script creates a log file, which you can follow for errors that might happen during the creation of elements such as istio-cni, istio-gateways, etc.

The log file is created under the execution directory, which is set by --istio.test.work_dir. You can also see the iop-sail.yaml file that has the Sail Operator configuration converted from Istio Operator control plane configuration.

The following is an example of the folder where you can find artifacts created in test execution:
https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift-service-mesh_istio/374/pull-ci-openshift-service-mesh-istio-master-istio-integration-sail-pilot/1920053129898889216/artifacts/istio-integration-sail-pilot/integ-sail-pilot-test-run/artifacts/pilot-4def8a9fdff144de8e4f22463/_suite_context/istio-deployment-611939208/

### Debugging test with dlv + vscode:
#### Prerequisites:
    - To debug test/s in integration test suite you need to have Sail Operator installed in the cluster

#### Executing Debugger
    - Put your breakpoints on desired test in vscode
    - Add following launch config to .vscode/launch.json
        ```json
        {
            "version": "0.2.0",
            "configurations": [
                {
                    "name": "Attach to Delve",
                    "type": "go",
                    "request": "attach",
                    "mode": "remote",
                    "port": 2345,
                    "host": "127.0.0.1",
                    "apiVersion": 2
                }
            ]
        }
        ```
    - Execute following dlv command on terminal with modifying -test.run as desired
        ```sh
        dlv test --headless --listen=:2345 --api-version=2 --log --build-flags "-tags=integ" -- \
        -test.v -test.count=1 -test.timeout=60m -test.run TestTraffic/externalname/routed/auto-http \
        --istio.test.ci \
        --istio.test.pullpolicy=IfNotPresent \
        --istio.test.work_dir=/home/ubuntu/istio_ossm/prow/artifacts \
        --istio.test.skipTProxy=true \
        --istio.test.skipVM=true \
        --istio.test.kube.helm.values=global.platform=openshift \
        --istio.test.istio.enableCNI=true \
        --istio.test.hub=image-registry.openshift-image-registry.svc:5000/istio-system \
        --istio.test.tag=istio-testing \
        --istio.test.openshift \
        --istio.test.kube.deploy=false \
        --istio.test.kube.controlPlaneInstaller=/home/ubuntu/istio_ossm/prow/setup/sail-operator-setup.sh
        ```
    - When the dlv command starts go to vscode and execute "Attach to Delve" debugger
