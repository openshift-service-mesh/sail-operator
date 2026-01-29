# Running Istio Integration Tests With Sail Operator

As the OpenShift Service Mesh (OSSM) evolves, it's critical to validate that Istio continues to function correctly when deployed using the Sail Operator, which is the default control plane installer for OSSM 3.x. However, the upstream Istio integration test framework uses Helm to perform Istio deployment. This misalignment creates challenges in testing OSSM features and regressions within the upstream test suites.

To bridge this gap, the integration test framework in upstream Istio has been extended to support running tests with Sail Operator. This allows developers and QE teams to confidently run comprehensive upstream test coverage in a Sail-managed environment on an OpenShift cluster. Using the Sail Operator ensures upstream compatibility, product quality, and stability for future OSSM releases.

## What is the Converter?

The converter is a component that translates Istio Operator (IstioOperator/IOP) configurations used by upstream Istio tests into Sail Operator custom resource configurations (Istio, IstioCNI, etc.). This enables the upstream test suite to deploy and manage Istio control planes using Sail Operator instead of Helm, allowing validation of OSSM-specific deployment patterns.

## Prerequisites

- OpenShift cluster with cluster-admin access
- OpenShift Service Mesh Operator 3.1 or later (if using preinstalled OSSM)
- Local clone of the [service-mesh-istio](https://github.com/openshift-service-mesh/istio) repository
  ```sh
  git clone https://github.com/openshift-service-mesh/istio.git
  cd istio
  ```

## Environment Setup

Before running the converter with the Istio testing framework, you must configure your environment. You have two options for setting up the Sail Operator:

1. Use an already installed OSSM
2. Deploy the Sail Operator from source during test execution

Each approach requires a different set of environment variables, as explained below.

### Option 1: Using Preinstalled OSSM

If you already have OSSM (OpenShift Service Mesh) preinstalled, set the following environment variables:

```sh
export CONTROL_PLANE_SOURCE=sail
export TEST_HUB=docker.io/istio
export TAG=1.26.2  # Modify for desired version
export ISTIO_VERSION=v1.26.2  # Modify for desired version
```

### Option 2: Deploying Sail Operator from Source

If you want the test runner to install the Sail Operator from source, set:

```sh
export CONTROL_PLANE_SOURCE=sail
export INSTALL_SAIL_OPERATOR=true
```

This instructs the test runner script to deploy the Sail Operator first and execute tests with the supported control plane version.

#### Optional Environment Variables

You may also define the following optional variables to control which Istio version is installed via the Sail Operator:

- **CONVERTER_BRANCH**: Specifies a Sail Operator release branch. The converter will fetch the latest supported Istio version from:
  ```
  https://raw.githubusercontent.com/istio-ecosystem/sail-operator/$CONVERTER_BRANCH/pkg/istioversion/versions.yaml
  ```

  **Default:** `main`

  **Example:**
  ```sh
  export CONVERTER_BRANCH=release-3.1
  ```

## Executing Tests

### Running Test Suites

#### Pilot Test Suite

To run the pilot test suite, execute:

```sh
prow/integ-suite-ocp.sh pilot 'TestCNIVersionSkew|TestGateway|TestAuthZCheck|TestKubeInject|TestRevisionTags|TestUninstallByRevision|TestUninstallWithSetFlag|TestUninstallCustomFile|TestUninstallPurge|TestCNIRaceRepair|TestValidation|TestWebhook|TestMultiRevision'
```

**Note:** Some tests are currently skipped due to OpenShift-specific issues tracked in [OSSM-9328](https://issues.redhat.com/browse/OSSM-9328). This documentation will be updated once the issue is resolved.

#### Telemetry Test Suite

To run the telemetry test suite, execute:

```sh
prow/integ-suite-ocp.sh telemetry
```

### Interpreting Test Results

- **Success**: Tests will exit with status code 0 and display "PASS" messages
- **Failure**: Tests will exit with non-zero status and display "FAIL" messages with stack traces
- **Artifacts**: Test artifacts (logs, configurations) are stored in the directory specified by `--istio.test.work_dir`

## Debugging

### Debugging with Converter Logs

Every execution of the converter script creates a log file, which you can examine for errors that might occur during the creation of resources such as IstioCNI, Istio gateways, etc.

**Log Location:** The log file is created under the execution directory specified by `--istio.test.work_dir`

**Key Artifacts:**
- **Converter logs**: Detailed conversion and deployment logs
- **iop-sail.yaml**: The Sail Operator configuration converted from Istio Operator (IOP) configuration

**Example Artifacts Directory:**

[CI artifacts example](https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift-service-mesh_istio/374/pull-ci-openshift-service-mesh-istio-master-istio-integration-sail-pilot/1920053129898889216/artifacts/istio-integration-sail-pilot/integ-sail-pilot-test-run/artifacts/pilot-4def8a9fdff144de8e4f22463/_suite_context/istio-deployment-611939208/)

### Debugging Tests with Delve and VSCode

#### Prerequisites

- Sail Operator installed in the cluster
- VSCode with the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go) installed
- Latest Delve debugger installed:
  ```sh
  go install github.com/go-delve/delve/cmd/dlv@latest
  ```
- Verify Delve installation:
  ```sh
  dlv version  # Should show v1.23.0 or later for Go 1.24+
  ```
- Ensure `$HOME/go/bin` is in your PATH before other Go bin directories

You can debug tests using two approaches:
1. Run tests externally via terminal and attach the debugger
2. Run and debug tests directly inside VSCode

#### Approach 1: Command Line Execution with Debugger Attachment

1. **Set breakpoints** in VSCode on your desired test

2. **Add launch configuration** to `.vscode/launch.json`:

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

3. **Navigate to the test package directory** (important: `dlv test` must run from within the test package):

   ```sh
   cd ./tests/integration/pilot
   ```

4. **Start Delve in headless mode**:

   Update the paths for `controlPlaneInstaller` and `istio.test.work_dir` to match your local setup and update `-test.run` with the desired test.
   Also, do not forget to add additional args if needed, e.g. `--istio.test.ambient` for ambient tests, or extend helm values if necessary. (which is done by `integ-suite-ocp.sh` script)
   ```sh
   dlv test --headless --listen=:2345 --api-version=2 --log \
     --build-flags "-tags=integ" -- \
     -test.v \
     -test.count=1 \
     -test.timeout=60m \
     -test.run TestTraffic/externalname/routed/auto-http \
     --istio.test.ci \
     --istio.test.pullpolicy=IfNotPresent \
     --istio.test.work_dir=<your-istio-repo-path>/artifacts \
     --istio.test.skipTProxy=true \
     --istio.test.skipVM=true \
     --istio.test.kube.helm.values=global.platform=openshift \
     --istio.test.istio.enableCNI=true \
     --istio.test.hub=image-registry.openshift-image-registry.svc:5000/istio-system \
     --istio.test.tag=istio-testing \
     --istio.test.openshift \
     --istio.test.kube.deploy=false \
     --istio.test.kube.controlPlaneInstaller=<your-istio-repo-path>/setup/sail-operator-setup.sh
   ```

5. **Attach debugger in VSCode**:
   - Open `Run and Debug` view (Ctrl+Shift+D or Cmd+Shift+D on macOS)
   - Select `Attach to Delve` from the dropdown
   - Press F5 or click the green play button to start debugging

#### Approach 2: Direct Debugging in VSCode with Selected Test

This approach allows you to select a test function name in VSCode and debug it directly without running commands in the terminal.

1. **Set breakpoints** in VSCode on your desired test

2. **Add launch configuration** to `.vscode/launch.json`:

   Update paths for `controlPlaneInstaller` and `istio.test.work_dir` to match your local setup.

   **Important Configuration Notes:**
   - **Test-specific arguments**: Add suite-specific flags as needed (e.g., `--istio.test.ambient` for ambient tests)
   - **Helm values**: Extend `--istio.test.kube.helm.values` if required (e.g., ambient tests need `pilot.trustedZtunnelNamespace=ztunnel`)
   - **Environment variables**: Set the `env` section with variables required by `sail-operator-setup.sh` (see example below)
   - **Reference**: Check what `integ-suite-ocp.sh` <ins>uses for your specific test suite and replicate those settings here</ins>. Also some additional setting can be needed. (e.g. for ambient, you need to Set local gateway mode for Ambient.)

   **How this configuration works:**
   - `${fileDirname}` - Automatically uses the directory of the currently open test file as the test package
   - `${selectedText}` - Uses the text you select/highlight in VSCode as the test name to run

   ```json
   {
       "version": "0.2.0",
       "configurations": [
           {
               "name": "Debug Selected Test",
               "type": "go",
               "request": "launch",
               "mode": "test",
               "program": "${fileDirname}",
               "args": [
                   "-test.v",
                   "-test.count=1",
                   "-test.timeout=60m",
                   "-test.run", "${selectedText}",
                   "--istio.test.ci",
                   "--istio.test.pullpolicy=IfNotPresent",
                   "--istio.test.work_dir=<your-istio-repo-path>/prow/artifacts",
                   "--istio.test.skipTProxy=true",
                   "--istio.test.skipVM=true",
                   "--istio.test.kube.helm.values=global.platform=openshift,pilot.trustedZtunnelNamespace=ztunnel",
                   "--istio.test.istio.enableCNI=true",
                   "--istio.test.kube.deployGatewayAPI=false",
                   "--istio.test.kube.deploy=false",
                   "--istio.test.hub=docker.io/istio",
                   "--istio.test.tag=1.27.3",
                   "--istio.test.openshift",
                   "--istio.test.ambient",
                   "--istio.test.kube.deploy=false",
                   "--istio.test.kube.controlPlaneInstaller=<your-istio-repo-path>/prow/setup/sail-operator-setup.sh"
               ],
               "buildFlags": "-tags=integ",
               "env": {
                   "CONTROL_PLANE_SOURCE": "sail",
                   "ISTIO_VERSION": "v1.27.3",
                   "TAG": "1.27.3",
                   "TEST_HUB": "docker.io/istio",
                   "AMBIENT": "true"
               }
           }
       ]
   }
   ```

3. **Run the debugger**:
   1. Open the test file in VSCode (e.g., `traffic_test.go`)
   2. Double-click to select the test function name (e.g., `TestTraffic`)
   3. Open `Run and Debug` view (Ctrl+Shift+D or Cmd+Shift+D on macOS)
   4. Select `Debug Selected Test` from the dropdown
   5. Press F5 or click the green play button to start debugging
