# nmcrun - RunAI Log Collector

A comprehensive tool for collecting logs and environment details from RunAI Kubernetes deployments. Converts your shell script into a cross-platform Go binary with versioning and auto-update capabilities.

## Features

- 🚀 **Cross-platform binary** - Works on macOS, Linux, and Windows
- 📦 **Log collection** - Gathers pod logs, configuration, and cluster information
- 🔄 **Auto-update** - Built-in version checking and upgrade functionality
- 📊 **Comprehensive reporting** - Collects Helm charts, ConfigMaps, and cluster state
- 🗜️ **Archive creation** - Automatically creates timestamped tar.gz archives
- 🏷️ **Version tracking** - Know exactly which version your customers are running

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

# Check version information
nmcrun version

# Check for updates and upgrade
nmcrun upgrade

# Show help
nmcrun --help
```

### Environment Testing

The `nmcrun test` command verifies your environment before log collection:

- ✅ **Tool availability**: Checks if `kubectl` and `helm` are installed and working
- 🌐 **Cluster connectivity**: Tests if `kubectl` can connect to your cluster
- 📋 **Namespace verification**: Checks if RunAI namespaces (`runai`, `runai-backend`) exist
- 📊 **RunAI information**: Displays cluster URL, control plane URL, RunAI version, and cluster version
- 👥 **Permissions check**: Verifies if you have sufficient cluster permissions

Run `nmcrun test` before collecting logs to ensure everything is properly configured.

### What Gets Collected

The tool collects the following information from your Kubernetes cluster:

#### For `runai` namespace:
- Pod logs (regular and init containers)
- Helm charts list
- Helm values for runai-cluster
- ConfigMap runai-public
- Pod lists
- Node information
- RunAI configuration
- Engine configuration

#### For `runai-backend` namespace:
- Pod logs (regular and init containers)
- Pod lists
- Helm values for runai-backend

#### Output Structure:
```
{controlplane-name}-{namespace}-logs-{timestamp}/
├── logs/
│   ├── {pod}_{container}.log
│   └── {pod}_{container}_init.log
├── script.log
├── helm_charts_list.txt
├── helm-values_runai-cluster.yaml
├── cm_runai-public.yaml
├── pod-list_runai.txt
├── node-list.txt
├── runaiconfig.yaml
└── engine-config.yaml
```

## Development

### Prerequisites

- Go 1.21 or later
- kubectl (for log collection functionality)
- helm (for Helm-related data collection)

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
   - `kubectl` configured for your cluster
   - `helm` installed
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

## Troubleshooting

### Common Issues

1. **"kubectl command not found"** (`nmcrun test` fails)
   - Install kubectl and ensure it's in your PATH
   - Verify installation with `kubectl version --client`

2. **"helm command not found"** (`nmcrun test` fails)
   - Install Helm: https://helm.sh/docs/intro/install/
   - Verify installation with `helm version`

3. **"kubectl cannot connect to cluster"** (`nmcrun test` fails)
   - Check your kubeconfig: `kubectl config current-context`
   - Verify cluster connectivity: `kubectl get nodes`
   - Ensure you're connected to the correct cluster

4. **"no RunAI namespaces found"** (`nmcrun test` fails)
   - Verify RunAI is installed: `kubectl get namespaces | grep runai`
   - Check if you're connected to the correct cluster
   - Ensure RunAI installation is complete

5. **Permission errors** (during log collection)
   - Ensure your kubectl context has read access to the required namespaces
   - Check RBAC permissions for the service account
   - Run `nmcrun test` to verify permissions

6. **Upgrade fails**
   - Check internet connectivity
   - Verify the GitHub repository is accessible
   - Ensure write permissions to the binary location

7. **macOS Security Warning ("cannot be opened because the developer cannot be verified")**
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
