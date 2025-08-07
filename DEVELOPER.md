# nmcrun - Code Migration Guide

## Overview

`nmcrun` is a Go-based CLI tool for collecting RunAI cluster diagnostics using native Kubernetes client libraries. This guide explains how to migrate the core functionality into your existing Go CLI, allowing you to deprecate the standalone tool while preserving all diagnostic capabilities.

## 🏗️ Architecture

### Core Design Principles

- **Zero External Dependencies**: Pure Go implementation using `client-go`
- **Modular Architecture**: Clean separation between CLI interface and collection logic
- **Native Kubernetes Integration**: Direct API calls, no kubectl/helm dependencies
- **Flexible Authentication**: Multiple auth methods with automatic fallbacks

### Project Structure

```
nmcrun/
├── main.go                    # CLI interface (Cobra commands)
├── internal/
│   ├── collector/            # Core collection logic
│   │   └── collector.go     # Main collector implementation
│   ├── updater/             # Update functionality
│   │   └── updater.go
│   └── version/             # Version information
│       └── version.go
├── go.mod                   # Dependencies
└── README.md               # User documentation
```

## 🔧 Migration Strategy

### Recommended Approach: Copy Core Functions

The cleanest migration path is to copy the core diagnostic functions into your existing CLI structure. This allows you to:

- 🗑️ **Deprecate** the standalone nmcrun tool
- 🎯 **Customize** the functionality for your specific needs
- 🔧 **Maintain** full control over the diagnostic code
- 📦 **Integrate** seamlessly with your existing error handling and workflows

### Migration Steps

#### Step 1: Copy Core Files

Copy these essential files from nmcrun into your CLI:

```
your-cli/
├── pkg/
│   └── diagnostics/
│       ├── collector.go        # Core collection logic
│       ├── authentication.go   # Kubernetes auth methods
│       ├── workloads.go        # Workload collection
│       ├── scheduler.go        # Scheduler resource collection
│       └── logs.go            # Log collection
```

#### Step 2: Extract Key Functions

From `internal/collector/collector.go`, extract these core functions:

**Essential Functions to Copy:**
```go
// Authentication and setup
func getKubernetesConfig() (*rest.Config, error)
func NewCollector() (*Collector, error)

// Core collection methods
func (c *Collector) CollectLogs() error
func (c *Collector) CollectWorkloadInfo(project, workloadType, name string) error
func (c *Collector) CollectSchedulerInfo() error
func (c *Collector) RunTests() error

// Kubernetes resource helpers
func (c *Collector) getResourceAsYAML(namespace, resource, name string) (string, error)
func (c *Collector) getPodsWithLabels(namespace, labelSelector string) (*corev1.PodList, error)
func (c *Collector) getPodGroupsWithLabels(namespace, labelSelector string) (*unstructured.UnstructuredList, error)
```

#### Step 3: Integrate into Your CLI Structure

```go
// In your CLI's diagnostics package
package diagnostics

import (
    // Your existing imports
    "context"
    "fmt"
    "time"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/rest"
    // ... other k8s imports
)

// Your collector struct (adapted from nmcrun)
type RunAICollector struct {
    clientset     *kubernetes.Clientset
    dynamicClient dynamic.Interface
    config        *rest.Config
    // Add your CLI-specific fields
    logger        YourLogger
    outputDir     string
}

// Adapted initialization
func NewRunAICollector(logger YourLogger) (*RunAICollector, error) {
    config, err := getKubernetesConfig() // Copy this function
    if err != nil {
        return nil, fmt.Errorf("failed to get k8s config: %w", err)
    }
    
    // Initialize clients (copy from nmcrun)
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }
    
    dynamicClient, err := dynamic.NewForConfig(config)
    if err != nil {
        return nil, err
    }
    
    return &RunAICollector{
        clientset:     clientset,
        dynamicClient: dynamicClient,
        config:        config,
        logger:        logger,
        outputDir:     "/tmp/diagnostics",
    }, nil
}
```

## 📦 Key Dependencies

Add these to your `go.mod`:

```go
require (
    github.com/spf13/cobra v1.8.0
    k8s.io/api v0.28.4
    k8s.io/apimachinery v0.28.4
    k8s.io/client-go v0.28.4
    sigs.k8s.io/yaml v1.3.0
)
```

## 🎯 Core Collector API

### Main Interface

```go
type Collector struct {
    // Internal fields for Kubernetes clients
    clientset     *kubernetes.Clientset
    dynamicClient dynamic.Interface
    config        *rest.Config
}

// Primary constructor
func New() (*Collector, error)

// Core collection methods
func (c *Collector) CollectLogs() error
func (c *Collector) CollectWorkloadInfo(project, workloadType, name string) error
func (c *Collector) CollectSchedulerInfo() error
func (c *Collector) RunTests() error
```

### Authentication Methods

The collector automatically tries these authentication methods in order:

1. **In-cluster** (service account)
2. **Kubeconfig file** (`~/.kube/config` or `KUBECONFIG`)
3. **Service account token file** (`/var/run/secrets/kubernetes.io/serviceaccount/token`)
4. **Environment variables** (`KUBERNETES_SERVICE_HOST`, `KUBERNETES_TOKEN`)

### Error Handling

```go
collector, err := collector.New()
if err != nil {
    // Handle authentication/connection errors
    return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
}

err = collector.CollectLogs()
if err != nil {
    // Handle collection errors
    return fmt.Errorf("log collection failed: %w", err)
}
```

## 🔌 CLI Integration Patterns

### Pattern 1: New Diagnostic Commands

Add RunAI diagnostics as new commands in your existing CLI:

```go
// In your CLI's command setup
func addDiagnosticsCommands(rootCmd *cobra.Command) {
    diagnosticsCmd := &cobra.Command{
        Use:   "diagnostics",
        Short: "RunAI cluster diagnostics",
    }
    
    // Logs command (migrated from nmcrun logs)
    logsCmd := &cobra.Command{
        Use:   "logs",
        Short: "Collect RunAI logs",
        RunE: func(cmd *cobra.Command, args []string) error {
            collector, err := diagnostics.NewRunAICollector(yourLogger)
            if err != nil {
                return err
            }
            return collector.CollectLogs()
        },
    }
    
    // Workload command (migrated from nmcrun workloads)
    workloadCmd := &cobra.Command{
        Use:   "workload",
        Short: "Collect workload diagnostics", 
        RunE: func(cmd *cobra.Command, args []string) error {
            project, _ := cmd.Flags().GetString("project")
            workloadType, _ := cmd.Flags().GetString("type")
            name, _ := cmd.Flags().GetString("name")
            
            collector, err := diagnostics.NewRunAICollector(yourLogger)
            if err != nil {
                return err
            }
            return collector.CollectWorkloadInfo(project, workloadType, name)
        },
    }
    
    diagnosticsCmd.AddCommand(logsCmd, workloadCmd)
    rootCmd.AddCommand(diagnosticsCmd)
}
```

### Pattern 2: Embedded in Existing Workflows

Integrate diagnostic collection into your existing deployment/management commands:

```go
// Enhance your existing deploy command
func deployWorkloadWithDiagnostics(project, name string) error {
    // Your existing deployment logic
    err := yourExistingDeployFunction(project, name)
    if err != nil {
        // Auto-collect diagnostics on deployment failure
        yourLogger.Info("Deployment failed, collecting diagnostics...")
        
        collector, diagErr := diagnostics.NewRunAICollector(yourLogger)
        if diagErr == nil {
            collector.CollectWorkloadInfo(project, "tw", name)
            yourLogger.Info("Diagnostics collected to assist with troubleshooting")
        }
        
        return fmt.Errorf("deployment failed: %w", err)
    }
    return nil
}

// Add to your status/health commands
func checkRunAIHealth() error {
    collector, err := diagnostics.NewRunAICollector(yourLogger)
    if err != nil {
        return fmt.Errorf("cannot connect to RunAI cluster: %w", err)
    }
    
    return collector.RunTests() // Uses the migrated test functionality
}
```

### Pattern 3: On-Demand Troubleshooting

Trigger diagnostic collection from any command when issues are detected:

```go
// Helper function available throughout your CLI
func collectDiagnosticsOnError(project, workloadName string, originalErr error) error {
    yourLogger.Info("Error detected, collecting diagnostics for troubleshooting...")
    
    collector, err := diagnostics.NewRunAICollector(yourLogger)
    if err != nil {
        yourLogger.Warn("Could not initialize diagnostics collector: %v", err)
        return originalErr
    }
    
    // Collect relevant diagnostics based on context
    if workloadName != "" {
        collector.CollectWorkloadInfo(project, "tw", workloadName)
    } else {
        collector.CollectLogs()
    }
    
    return fmt.Errorf("operation failed, diagnostics collected: %w", originalErr)
}
```

## 🛠️ Customization Options

### Custom Output Directories

```go
// Modify the collector to use custom output paths
collector, _ := collector.New()

// Set custom timestamp format
collector.SetTimestampFormat("2006-01-02_15-04-05")

// Set custom output directory
collector.SetOutputDir("/custom/output/path")
```

### Selective Collection

```go
// Collect only specific namespaces
collector.SetNamespaces([]string{"runai-custom", "runai-backend"})

// Collect only specific resource types
collector.SetResourceTypes([]string{"projects", "nodepools"})
```

### Custom Authentication

```go
// Use existing Kubernetes config
config := getYourExistingKubeConfig()
collector, err := collector.NewWithConfig(config)
```

## 📊 What Gets Collected

### Log Collection (`CollectLogs`)
- Pod logs from `runai` and `runai-backend` namespaces
- Helm release information (from Kubernetes secrets)
- ConfigMaps and cluster state
- Node information
- RunAI configuration

### Workload Collection (`CollectWorkloadInfo`)
- Workload YAML manifests
- Pod YAML and logs
- PodGroup YAML
- RunAIJob YAML
- KSVC YAML (for inference workloads)

### Scheduler Collection (`CollectSchedulerInfo`)
- Projects, Queues, Nodepools, Departments
- Individual resource YAML manifests
- Resource lists with metadata

## 🚀 Performance Considerations

### Concurrent Collection
The collector can be run concurrently:

```go
var wg sync.WaitGroup
for _, project := range projects {
    wg.Add(1)
    go func(p string) {
        defer wg.Done()
        collector, _ := collector.New()
        collector.CollectWorkloadInfo(p, "tw", "workload-name")
    }(project)
}
wg.Wait()
```

### Memory Usage
- Uses streaming for large log files
- Implements cleanup of temporary files
- Minimal memory footprint per collection

## 🧪 Testing Integration

### Unit Testing

```go
func TestRunAIDiagnostics(t *testing.T) {
    // Mock Kubernetes client for testing
    collector := &collector.Collector{
        // Initialize with test client
    }
    
    err := collector.RunTests()
    assert.NoError(t, err)
}
```

### Integration Testing

```go
func TestLiveCluster(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    collector, err := collector.New()
    require.NoError(t, err)
    
    err = collector.RunTests()
    assert.NoError(t, err)
}
```

## 🔒 Security Considerations

### RBAC Requirements

Minimum cluster permissions needed:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: runai-diagnostics
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "configmaps", "namespaces", "nodes", "secrets"]
  verbs: ["get", "list"]
- apiGroups: ["run.ai"]
  resources: ["*"]
  verbs: ["get", "list"]
- apiGroups: ["scheduling.run.ai"]
  resources: ["*"]
  verbs: ["get", "list"]
```

### Data Sensitivity
- Logs may contain sensitive information
- Implement appropriate data retention policies
- Consider log filtering for production environments

## 📈 Migration Timeline

### Phase 1: Code Migration (Week 1-2)
1. **Copy core functions** from `internal/collector/collector.go`
2. **Adapt authentication** functions to your existing auth patterns
3. **Extract resource collection** logic into your diagnostics package
4. **Test basic functionality** in development environment

### Phase 2: CLI Integration (Week 2-3)
1. **Add diagnostic commands** to your CLI using copied functions
2. **Integrate with existing logging/error handling** systems
3. **Customize output formats** to match your CLI's conventions
4. **Test end-to-end** workflows

### Phase 3: Enhanced Integration (Week 3-4)
1. **Embed diagnostics** into existing deployment/management workflows
2. **Add automated triggers** for diagnostic collection on failures
3. **Implement custom filters** for your specific use cases
4. **Update documentation** and user guides

### Phase 4: Deprecation (Week 4+)
1. **Announce deprecation** of standalone nmcrun tool
2. **Provide migration guide** for existing nmcrun users
3. **Archive nmcrun repository** once migration is complete
4. **Maintain diagnostic functionality** within your CLI

## 📋 Key Functions to Copy

### Essential Functions (Must Copy)

From `internal/collector/collector.go`, these are the critical functions your team should copy:

**Authentication & Setup:**
- `getKubernetesConfig()` - Multi-method Kubernetes authentication
- `tryKubeconfigAuth()`, `tryServiceAccountTokenAuth()`, `tryEnvironmentAuth()` - Auth fallbacks
- `New()` - Collector initialization

**Core Collection Methods:**
- `CollectLogs()` - Complete log collection workflow
- `CollectWorkloadInfo()` - Workload diagnostics
- `CollectSchedulerInfo()` - Scheduler resource collection  
- `RunTests()` - Environment validation

**Kubernetes Helpers:**
- `getResourceAsYAML()` - Generic resource YAML extraction
- `getPodsWithLabels()` - Pod discovery by labels
- `getPodGroupsWithLabels()` - PodGroup discovery  
- `getPodLogs()` - Container log extraction
- `dumpSchedulerResource()` - Scheduler resource dumping

**Utility Functions:**
- `validateFileContent()` - Empty file prevention
- `createWorkloadArchive()` - Archive creation
- `objectToYAML()` - Kubernetes object serialization

### Nice-to-Have Functions (Optional)

- Update functionality from `internal/updater/` (if you want auto-updates)
- Version information from `internal/version/` (for diagnostics)

## 🔄 Deprecation Strategy

### User Communication

1. **Announcement**: Notify existing nmcrun users about migration to your CLI
2. **Documentation**: Provide clear migration instructions
3. **Timeline**: Give users adequate time to migrate (e.g., 3-6 months)
4. **Support**: Offer support during migration period

### Example Deprecation Notice

```
DEPRECATION NOTICE: nmcrun standalone tool

The nmcrun functionality has been integrated into [your-cli-name].
Please migrate to using the new diagnostic commands:

Old: nmcrun logs
New: your-cli diagnostics logs

Old: nmcrun workloads -p project -t tw -n workload  
New: your-cli diagnostics workload --project project --type tw --name workload

Old: nmcrun scheduler
New: your-cli diagnostics scheduler

Migration guide: [link to your docs]
Support timeline: nmcrun will be archived on [date]
```

## 📞 Post-Migration Support

- **Internal Documentation**: Document the migrated functions for your team
- **User Guide**: Update your CLI documentation with diagnostic capabilities  
- **Issue Tracking**: Handle diagnostic-related issues in your existing support channels
- **Maintenance**: You now own and maintain the diagnostic functionality

---

*This guide helps you completely migrate nmcrun's diagnostic capabilities into your existing CLI, allowing you to deprecate the standalone tool while providing enhanced RunAI diagnostics to your users.*