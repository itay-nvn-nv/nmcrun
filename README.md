# nmcrun - RunAI Log Collector

A comprehensive tool for collecting logs and environment details from RunAI Kubernetes deployments. Converts your shell script into a cross-platform Go binary with versioning and auto-update capabilities.

## Features

- 🚀 **Cross-platform binary** - Works on macOS, Linux, and Windows
- 📦 **Log collection** - Gathers pod logs, configuration, and cluster information
- 🔄 **Auto-update** - Built-in version checking and upgrade functionality
- 📊 **Comprehensive reporting** - Collects Helm charts, ConfigMaps, and cluster state
- 🗜️ **Archive creation** - Automatically creates timestamped tar.gz archives
- 🏷️ **Version tracking** - Know exactly which version your customers are running
- 🔍 **Workload analysis** - Detailed information collection for specific workloads
- 🗂️ **Scheduler diagnostics** - Complete RunAI scheduler resource collection
- ⚡ **100% Native integration** - Uses Kubernetes Go client libraries exclusively (no external tools required)

## Installation

### Download Pre-built Binaries

Once you create a release, customers can download the appropriate binary for their platform:

- **macOS (Intel)**: `nmcrun_*_darwin_amd64.tar.gz`
- **macOS (Apple Silicon)**: `nmcrun_*_darwin_arm64.tar.gz`
- **Linux (x86_64)**: `nmcrun_*_linux_amd64.tar.gz`
- **Linux (ARM64)**: `nmcrun_*_linux_arm64.tar.gz`
- **Windows (x86_64)**: `nmcrun_*_windows_amd64.zip`

Extract the archive and place the binary in your PATH.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/itay-nvn-nv/nmcrun.git
cd nmcrun

# Build for current platform
make build

# Or build development version
make dev-build

# Install to system
make install
```

## Usage

### Basic Usage

```bash
# Show help (default action)
nmcrun

# Test environment and connectivity
nmcrun test

# Run log collection
nmcrun logs

# Collect workload information
nmcrun workloads --project myproject --type tw --name myworkload

# Collect scheduler information
nmcrun scheduler

# Check version information
nmcrun version

# Check for updates and upgrade
nmcrun upgrade

# Show help
nmcrun --help
```

### Environment Testing

The `nmcrun test` command verifies your environment before log collection:

- ✅ **System requirements**: Verifies all requirements are satisfied (no external tools required)
- 🌐 **Cluster connectivity**: Tests Kubernetes cluster connection using native client libraries
- 📋 **Namespace verification**: Checks if RunAI namespaces (`runai`, `runai-backend`) exist
- 📊 **RunAI information**: Displays cluster URL, control plane URL, RunAI version, and cluster version
- 👥 **Permissions check**: Verifies if you have sufficient cluster permissions

Run `nmcrun test` before collecting logs to ensure everything is properly configured.

### What's New: Zero External Dependencies

**nmcrun** is now completely self-contained with zero external tool dependencies! The application uses native Kubernetes Go client libraries (`client-go`) to communicate directly with your cluster for all operations including Helm release information extraction.

**Benefits:**
- ✅ **Zero external dependencies** - No need to install kubectl, helm, or any other tools
- ⚡ **Better performance** - Direct API calls are faster than external commands
- 🔒 **Enhanced security** - Uses your existing kubeconfig authentication directly  
- 🛠️ **Simplified deployment** - Single binary with everything built-in
- 📦 **100% self-contained** - Works anywhere Go can run

**What you need:**
- Kubernetes cluster authentication (see authentication methods below)
- Appropriate cluster permissions

### Authentication Methods

The tool supports multiple authentication methods and automatically tries them in order:

#### 1. **In-Cluster Authentication** (Automatic)
- When running inside a Kubernetes pod with a service account
- Uses the mounted service account token automatically
- Perfect for running as a Job or CronJob in the cluster

#### 2. **Kubeconfig File** (Most Common)
- Standard kubectl configuration file
- Automatically detected from:
  - `~/.kube/config` (default location)
  - `KUBECONFIG` environment variable
  - `--kubeconfig` flag locations

#### 3. **Service Account Token File**
- Direct service account token authentication
- Looks for token at `/var/run/secrets/kubernetes.io/serviceaccount/token`
- Useful for containerized environments

#### 4. **Environment Variables**
- Manual configuration via environment variables:
  ```bash
  export KUBERNETES_SERVICE_HOST=your-cluster-api-server
  export KUBERNETES_SERVICE_PORT=443
  export KUBERNETES_TOKEN=your-bearer-token
  export KUBERNETES_CA_CERT_FILE=/path/to/ca.crt  # optional
  ```

**For existing users**: You can now safely remove kubectl and helm if they were only being used for nmcrun. The tool will continue to work exactly the same way using your existing kubeconfig file.

### Usage Examples for Different Environments

#### 🖥️ **Local Development/Admin Use**
```bash
# Uses ~/.kube/config automatically
nmcrun test
nmcrun logs
```

#### 🐳 **Running in a Kubernetes Pod**
```yaml
# Kubernetes Job example
apiVersion: batch/v1
kind: Job
metadata:
  name: nmcrun-collector
spec:
  template:
    spec:
      serviceAccountName: nmcrun-service-account
      containers:
      - name: nmcrun
        image: your-registry/nmcrun:latest
        command: ["nmcrun", "logs"]
      restartPolicy: Never
```

#### 🔑 **Using Service Account Token**
```bash
# Set up environment variables
export KUBERNETES_SERVICE_HOST=my-cluster.example.com
export KUBERNETES_SERVICE_PORT=443
export KUBERNETES_TOKEN=$(cat /path/to/service-account.token)

# Run nmcrun
nmcrun test
```

#### 🌐 **Custom Kubeconfig Location**
```bash
# Point to specific kubeconfig file
export KUBECONFIG=/path/to/my/kubeconfig
nmcrun logs
```

### Service Account Setup for In-Cluster Usage

When running nmcrun inside a Kubernetes cluster (as a Job, CronJob, or regular pod), you'll need a service account with appropriate permissions:

```yaml
# service-account.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: nmcrun-service-account
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nmcrun-reader
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "namespaces", "configmaps", "nodes"]
  verbs: ["get", "list"]
- apiGroups: ["run.ai"]
  resources: ["*"]
  verbs: ["get", "list"]
- apiGroups: ["scheduling.k8s.io"]
  resources: ["*"]
  verbs: ["get", "list"]
- apiGroups: ["serving.knative.dev"]
  resources: ["*"]
  verbs: ["get", "list"]
- apiGroups: ["engine.run.ai"]
  resources: ["*"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: nmcrun-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: nmcrun-reader
subjects:
- kind: ServiceAccount
  name: nmcrun-service-account
  namespace: default
```

Apply with:
```bash
kubectl apply -f service-account.yaml
```

## kubectl Command Equivalents

For troubleshooting, manual verification, or understanding what nmcrun does under the hood, here are the equivalent kubectl commands for each operation. These are useful when:

- 🐛 **Debugging**: nmcrun fails and you want to test manually
- 🔍 **Understanding**: You want to see exactly what data is being collected  
- ⚡ **Quick checks**: You need to verify specific resources quickly
- 📚 **Learning**: You want to understand the Kubernetes operations involved

**Note**: These commands show what nmcrun does internally using the Kubernetes API. nmcrun automates all of these steps and handles errors gracefully.

### Environment Testing (`nmcrun test`)

```bash
# Test cluster connectivity
kubectl cluster-info
kubectl get nodes

# Check RunAI namespaces
kubectl get namespaces | grep runai

# Get RunAI configuration and version
kubectl -n runai get runaiconfig runai -o yaml
kubectl -n runai get configmap runai-public -o jsonpath='{.data.cluster-version}'

# Check cluster context
kubectl config current-context

# Verify permissions
kubectl auth can-i get pods --all-namespaces
kubectl auth can-i get pods -n runai
kubectl auth can-i get pods -n runai-backend
```

### Log Collection (`nmcrun logs`)

```bash
# List all pods in RunAI namespaces
kubectl get pods -n runai -o wide
kubectl get pods -n runai-backend -o wide

# Get pod logs (example for a specific pod)
kubectl logs -n runai <pod-name> --timestamps
kubectl logs -n runai <pod-name> -c <container-name> --timestamps

# Get all containers in a pod
kubectl get pod <pod-name> -n runai -o jsonpath='{.spec.containers[*].name}'
kubectl get pod <pod-name> -n runai -o jsonpath='{.spec.initContainers[*].name}'

# Collect configuration
kubectl -n runai get configmap runai-public -o yaml
kubectl -n runai get runaiconfig runai -o yaml
kubectl -n runai get configs.engine.run.ai engine-config -o yaml
kubectl get nodes -o wide

# Get Helm release information (from Kubernetes secrets)
kubectl get secrets -l owner=helm --all-namespaces
kubectl get secrets -l owner=helm -n runai
kubectl get secrets -l owner=helm -n runai-backend
```

### Workload Information (`nmcrun workloads`)

```bash
# Find namespace for a project
kubectl get ns -l runai/queue=<PROJECT_NAME> -o jsonpath='{.items[0].metadata.name}'

# Get workload YAML (replace <TYPE> with actual workload type)
kubectl -n <NAMESPACE> get <TYPE> <WORKLOAD_NAME> -o yaml

# Get RunAIJob YAML
kubectl -n <NAMESPACE> get runaijob <WORKLOAD_NAME> -o yaml

# Get pods for a workload
kubectl -n <NAMESPACE> get pod -l workloadName=<WORKLOAD_NAME> -o yaml

# Get PodGroup
kubectl -n <NAMESPACE> get podgroup -l workloadName=<WORKLOAD_NAME> -o yaml

# Get KSVC (for inference workloads)
kubectl -n <NAMESPACE> get ksvc <WORKLOAD_NAME> -o yaml

# Get pod logs for workload
kubectl -n <NAMESPACE> get pod -l workloadName=<WORKLOAD_NAME> -o jsonpath='{.items[*].metadata.name}'
kubectl -n <NAMESPACE> logs <POD_NAME> -c <CONTAINER_NAME>
```

### Scheduler Information (`nmcrun scheduler`)

```bash
# Get scheduler resources
kubectl get projects
kubectl get projects -o yaml

kubectl get queues  
kubectl get queues -o yaml

kubectl get nodepools
kubectl get nodepools -o yaml

kubectl get departments
kubectl get departments -o yaml

# Get individual resources (example)
kubectl get project <PROJECT_NAME> -o yaml
kubectl get queue <QUEUE_NAME> -o yaml
```

### Authentication Testing

```bash
# Test different authentication methods manually

# 1. Test current kubeconfig
kubectl config view
kubectl config current-context
kubectl get pods --all-namespaces --limit=1

# 2. Test with specific kubeconfig
KUBECONFIG=/path/to/config kubectl get pods

# 3. Test with service account token
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
kubectl --token="$TOKEN" --server="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT" get pods

# 4. Test with custom token
kubectl --token="$YOUR_TOKEN" --server="https://your-cluster-api" get pods
```

### Common Diagnostic Commands

```bash
# Check resource permissions
kubectl auth can-i --list
kubectl auth can-i get pods -n runai
kubectl auth can-i get logs -n runai

# Check API resources availability
kubectl api-resources | grep -E "(runai|scheduling|knative)"

# Verify custom resources exist
kubectl get crd | grep -E "(runai|scheduling|knative)"

# Check cluster resource usage
kubectl top nodes
kubectl top pods -n runai
kubectl top pods -n runai-backend

# Debug networking
kubectl get endpoints -n runai
kubectl get services -n runai
```

### Real-World Examples

#### Example: Manually collect logs for a specific pod
```bash
# 1. List pods in runai namespace
kubectl get pods -n runai

# 2. Get containers in a specific pod (example: runai-pod-12345)
kubectl get pod runai-pod-12345 -n runai -o jsonpath='{.spec.containers[*].name}'

# 3. Get logs for each container
kubectl logs runai-pod-12345 -n runai -c container1 --timestamps
kubectl logs runai-pod-12345 -n runai -c container2 --timestamps
```

#### Example: Debug a specific workload manually
```bash
# 1. Find the namespace for project "myproject"
NAMESPACE=$(kubectl get ns -l runai/queue=myproject -o jsonpath='{.items[0].metadata.name}')
echo "Project namespace: $NAMESPACE"

# 2. Check if workload exists
kubectl -n $NAMESPACE get trainingworkload myworkload

# 3. Get workload details
kubectl -n $NAMESPACE get trainingworkload myworkload -o yaml

# 4. Get associated pods
kubectl -n $NAMESPACE get pod -l workloadName=myworkload

# 5. Get pod logs
POD_NAME=$(kubectl -n $NAMESPACE get pod -l workloadName=myworkload -o jsonpath='{.items[0].metadata.name}')
kubectl -n $NAMESPACE logs $POD_NAME
```

#### Example: Check RunAI installation health
```bash
# Check if RunAI CRDs are installed
kubectl get crd | grep run.ai

# Check RunAI components status
kubectl get pods -n runai
kubectl get pods -n runai-backend

# Verify RunAI configuration
kubectl -n runai get runaiconfig runai -o yaml
```

### Workload Information Collection

The `nmcrun workloads` command collects detailed information about a specific RunAI workload:

```bash
nmcrun workloads --project myproject --type tw --name myworkload
```

**Parameters:**
- `--project` (`-p`): RunAI project name (required)
- `--type` (`-t`): Workload type (required). Valid values:
  - `tw` or `trainingworkloads` - Training workloads
  - `iw` or `interactiveworkloads` - Interactive workloads
  - `infw` or `inferenceworkloads` - Inference workloads
  - `dw` or `distributedworkloads` - Distributed training workloads
  - `dinfw` or `distributedinferenceworkloads` - Distributed inference workloads
  - `ew` or `externalworkloads` - External workloads
- `--name` (`-n`): Workload name (required)

**What gets collected:**
- Workload YAML manifest
- RunAIJob YAML
- Pod YAML (all pods for the workload)
- PodGroup YAML
- Pod logs from all containers
- KSVC YAML (for inference workloads only)

Creates an archive: `{project}_{type}_{workload}_{timestamp}.tar.gz`

### Scheduler Information Collection

The `nmcrun scheduler` command collects comprehensive RunAI scheduler information:

```bash
nmcrun scheduler
```

**What gets collected:**
- Projects: List and individual YAML manifests
- Queues: List and individual YAML manifests  
- Nodepools: List and individual YAML manifests
- Departments: List and individual YAML manifests

Creates an archive: `scheduler_info_dump_{timestamp}.tar.gz`

### What Gets Collected

The tool collects the following information from your Kubernetes cluster:

#### For `runai` namespace:
- Pod logs (regular and init containers)
- Helm release information (extracted from Kubernetes secrets)
- ConfigMap runai-public
- Pod lists
- Node information
- RunAI configuration
- Engine configuration

#### For `runai-backend` namespace:
- Pod logs (regular and init containers)
- Pod lists
- Helm release information (extracted from Kubernetes secrets)

#### Output Structure:
```
{controlplane-name}-{namespace}-logs-{timestamp}/
├── logs/
│   ├── {pod}_{container}.log
│   └── {pod}_{container}_init.log
├── script.log
├── helm_releases_info.txt
├── cm_runai-public.yaml
├── pod-list_runai.txt
├── node-list.txt
├── runaiconfig.yaml
└── engine-config.yaml
```

## Development

### Prerequisites

- Go 1.21 or later
- A Kubernetes cluster with a valid kubeconfig

**Note**: No external tools required! The tool uses native Kubernetes Go client libraries for everything including Helm release information extraction.

### Development Commands

```bash
# Show available make targets
make help

# Build for development
make dev-build

# Run tests
make test

# Build all platform binaries
make release

# Clean build artifacts
make clean
```

### Repository Structure

```
nmcrun/
├── main.go                     # CLI entry point
├── internal/
│   ├── collector/              # Log collection logic
│   ├── updater/               # Auto-update functionality
│   └── version/               # Version information
├── .github/workflows/         # GitHub Actions for releases
├── build.sh                   # Cross-platform build script
├── Makefile                   # Development tasks
└── README.md                  # This file
```

## Release Process

### Creating a Release

1. **Tag a version**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0 - Initial release"
   git push origin v1.0.0
   ```

2. **GitHub Actions automatically**:
   - Builds binaries for all platforms
   - Creates a GitHub release
   - Uploads platform-specific archives
   - Generates release notes

### Version Management

- Version information is embedded at build time using Go's ldflags
- The `nmcrun version` command shows:
  - Version number (from git tags)
  - Build date
  - Git commit hash
- The `nmcrun upgrade` command checks GitHub releases for updates

### Update Repository Settings

Before using auto-update functionality, update the repository information in `internal/updater/updater.go`:

```go
// Replace with your GitHub username and repository name
repoOwner: "your-github-username",
repoName:  "your-repository-name",
```

## Customer Instructions

Send these instructions to your customers:

### Installation

1. Download the appropriate binary for your platform from the [releases page](https://github.com/itay-nvn-nv/nmcrun/releases)
2. Extract the archive: `tar -xzf nmcrun_*_your_platform.tar.gz` (or unzip for Windows)
3. Make it executable and fix macOS security (Unix/macOS only):
   ```bash
   chmod +x nmcrun
   # On macOS, remove quarantine attribute to avoid security warning:
   xattr -d com.apple.quarantine nmcrun 2>/dev/null || true
   ```
4. Move to PATH: `sudo mv nmcrun /usr/local/bin/` (or add to your PATH)

### Usage

1. **Ensure prerequisites**:
   - Kubernetes cluster access (kubeconfig file, service account, or environment variables)
   - Appropriate cluster access permissions

2. **Test your environment** (optional but recommended):
   ```bash
   nmcrun test
   ```

3. **Run the collector**:
   ```bash
   nmcrun logs
   ```

3. **Check version**:
   ```bash
   nmcrun version
   ```

4. **Stay updated**:
   ```bash
   nmcrun upgrade
   ```

5. **Send the archive**: The tool creates a `.tar.gz` file with all collected logs and information.

## Security Considerations

- The tool only reads cluster information, never modifies anything
- All data is collected locally and archived for manual transmission
- No data is transmitted automatically over the network (except for version checks)
- The upgrade functionality downloads from GitHub releases only
- Uses native Kubernetes client libraries with your existing kubeconfig authentication
- Zero external tool dependencies (completely self-contained)

## Troubleshooting

### Common Issues

1. **"no valid Kubernetes authentication method found"** (`nmcrun` fails to start)
   - **No kubeconfig**: Set `KUBECONFIG` environment variable or place config at `~/.kube/config`
   - **No service account**: Ensure running in a pod with mounted service account token
   - **Missing environment variables**: Set `KUBERNETES_SERVICE_HOST`, `KUBERNETES_SERVICE_PORT`, and optionally `KUBERNETES_TOKEN`
   - **File permissions**: Check that kubeconfig or token files are readable

2. **"No namespace found for project"** (`nmcrun workloads` fails)
   - Verify the project name is correct
   - Check that the RunAI project exists in the cluster
   - Ensure you have access to the cluster where the project is deployed
   - **Manual check**: `kubectl get ns -l runai/queue=<PROJECT_NAME>`

3. **"Invalid workload type"** (`nmcrun workloads` fails)
   - Use valid type aliases: `tw`, `iw`, `infw`, `dw`, `dinfw`, `ew`
   - Or use full names: `trainingworkloads`, `interactiveworkloads`, etc.

4. **"Failed to get workload/resource"** (workloads/scheduler commands)
   - Verify the workload name exists in the specified project
   - Check cluster permissions for the resource types
   - Ensure RunAI is properly installed and resources exist
   - **Manual check**: `kubectl -n <NAMESPACE> get <WORKLOAD_TYPE> <WORKLOAD_NAME>`

5. **System requirements check fails**
   - This shouldn't happen as no external tools are required
   - Contact support if you see this error

6. **"cannot connect to cluster"** (`nmcrun test` fails)
   - Check your kubeconfig current context
   - Verify cluster connectivity and authentication
   - Ensure you're connected to the correct cluster
   - **Manual check**: `kubectl cluster-info` and `kubectl get nodes`

7. **"no RunAI namespaces found"** (`nmcrun test` fails)
   - Verify RunAI is installed in the cluster
   - Check if you're connected to the correct cluster
   - Ensure RunAI installation is complete
   - **Manual check**: `kubectl get namespaces | grep runai` and `kubectl get crd | grep run.ai`

8. **Permission errors** (during log collection)
   - Ensure your kubeconfig has read access to the required namespaces
   - Check RBAC permissions for your user account or service account
   - Run `nmcrun test` to verify permissions
   - **Manual check**: `kubectl auth can-i get pods -n runai` and `kubectl auth can-i get logs -n runai`

9. **Upgrade fails**
   - Check internet connectivity
   - Verify the GitHub repository is accessible
   - Ensure write permissions to the binary location

10. **macOS Security Warning ("cannot be opened because the developer cannot be verified")**
   - **Command line fix**: `xattr -d com.apple.quarantine nmcrun`
   - **GUI method**: Right-click → Open → Click "Open" when prompted
   - **System Settings**: Privacy & Security → Click "Open Anyway"

### Debug Mode

For development, you can modify the collector to add more verbose logging or adjust collection parameters.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Build and test: `make test && make dev-build`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
