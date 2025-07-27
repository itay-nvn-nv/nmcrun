package collector

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Collector struct {
	namespaces []string
	logDir     string
	timestamp  string
}

// New creates a new collector instance
func New() *Collector {
	return &Collector{
		namespaces: []string{"runai-backend", "runai"},
		timestamp:  time.Now().Format("02-01-2006_15-04"),
	}
}

// Run executes the log collection process
func (c *Collector) Run() error {
	fmt.Println("🚀 Starting RunAI log collection...")

	// Check required tools
	if err := c.checkRequiredTools(); err != nil {
		return fmt.Errorf("required tools check failed: %w", err)
	}

	// Extract cluster information
	clusterURL, cpURL, err := c.extractClusterInfo()
	if err != nil {
		fmt.Printf("⚠ Warning: Could not extract cluster information: %v\n", err)
		clusterURL = "unknown"
		cpURL = "unknown"
	}

	cpNameClean := c.cleanControlPlaneName(cpURL)

	fmt.Printf("Cluster URL: %s\n", clusterURL)
	fmt.Printf("Control Plane URL: %s\n", cpURL)
	fmt.Printf("Control Plane Name (cleaned): %s\n", cpNameClean)
	fmt.Println("==========================================")

	// Process each namespace
	for _, namespace := range c.namespaces {
		fmt.Printf("\n🔍 Processing namespace: %s\n", namespace)
		fmt.Println("----------------------------------------")

		// Check if namespace exists
		if !c.namespaceExists(namespace) {
			fmt.Printf("❌ Namespace '%s' does not exist. Skipping.\n", namespace)
			continue
		}

		fmt.Printf("✓ Namespace '%s' exists. Starting log collection...\n", namespace)

		logName := fmt.Sprintf("%s-%s-logs-%s", cpNameClean, namespace, c.timestamp)
		logDir := fmt.Sprintf("./%s", logName)
		archiveName := fmt.Sprintf("%s.tar.gz", logName)

		if err := c.processNamespace(namespace, logDir, archiveName, clusterURL, cpURL); err != nil {
			fmt.Printf("❌ Error processing namespace %s: %v\n", namespace, err)
			continue
		}

		fmt.Printf("✓ Completed processing namespace: %s\n", namespace)
		fmt.Printf("Archive created: %s\n", archiveName)
		fmt.Println("==========================================")
	}

	fmt.Println("\n🎉 All namespaces processed successfully!")
	return nil
}

// checkRequiredTools verifies that required tools are available
func (c *Collector) checkRequiredTools() error {
	fmt.Println("Checking for required tools...")

	tools := []string{"kubectl", "helm"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("'%s' command not found. Please install %s and try again", tool, tool)
		}
	}

	fmt.Println("✓ All required tools are available")
	return nil
}

// extractClusterInfo gets cluster and control plane URLs
func (c *Collector) extractClusterInfo() (string, string, error) {
	clusterURL, err := c.runKubectl("-n", "runai", "get", "runaiconfig", "runai", "-o", "jsonpath={.spec.__internal.global.clusterURL}")
	if err != nil {
		clusterURL = "unknown"
	}

	cpURL, err := c.runKubectl("-n", "runai", "get", "runaiconfig", "runai", "-o", "jsonpath={.spec.__internal.global.controlPlane.url}")
	if err != nil {
		cpURL = "unknown"
	}

	return strings.TrimSpace(clusterURL), strings.TrimSpace(cpURL), nil
}

// cleanControlPlaneName cleans the control plane URL for use in filenames
func (c *Collector) cleanControlPlaneName(cpURL string) string {
	// Remove https:// and replace dots with dashes
	clean := strings.Replace(cpURL, "https://", "", 1)
	clean = strings.Replace(clean, ".", "-", -1)
	return clean
}

// namespaceExists checks if a namespace exists
func (c *Collector) namespaceExists(namespace string) bool {
	_, err := c.runKubectl("get", "namespace", namespace)
	return err == nil
}

// processNamespace handles log collection for a single namespace
func (c *Collector) processNamespace(namespace, logDir, archiveName, clusterURL, cpURL string) error {
	// Create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	scriptLogPath := filepath.Join(logDir, "script.log")
	scriptLog, err := os.Create(scriptLogPath)
	if err != nil {
		return fmt.Errorf("failed to create script log: %w", err)
	}
	defer scriptLog.Close()

	// Write header to script log
	c.writeScriptLogHeader(scriptLog, namespace, clusterURL, cpURL)

	// Collect pod logs
	fmt.Println("📋 === Collecting Pod Logs ===")
	fmt.Fprintln(scriptLog, "=== Collecting Pod Logs ===")
	if err := c.collectPodLogs(namespace, logDir, scriptLog); err != nil {
		fmt.Printf("⚠️  Warning: Error collecting pod logs: %v\n", err)
		fmt.Fprintf(scriptLog, "Warning: Error collecting pod logs: %v\n", err)
	}

	// Collect additional information based on namespace
	fmt.Println("\n📊 === Collecting Additional Information ===")
	fmt.Fprintln(scriptLog, "\n=== Collecting Additional Information ===")
	if err := c.collectAdditionalInfo(namespace, logDir, scriptLog); err != nil {
		fmt.Printf("⚠️  Warning: Error collecting additional info: %v\n", err)
		fmt.Fprintf(scriptLog, "Warning: Error collecting additional info: %v\n", err)
	}

	// Create archive
	fmt.Println("\n📦 === Creating Archive ===")
	fmt.Fprintln(scriptLog, "\n=== Creating Archive ===")
	if err := c.createArchive(logDir, archiveName, scriptLog); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Clean up temp directory
	if err := os.RemoveAll(logDir); err != nil {
		fmt.Printf("Warning: Failed to clean up temp directory: %v\n", err)
	}

	return nil
}

// writeScriptLogHeader writes the header information to the script log
func (c *Collector) writeScriptLogHeader(w io.Writer, namespace, clusterURL, cpURL string) {
	fmt.Fprintf(w, "=== Log Collection Started at %s ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(w, "Namespace: %s\n", namespace)
	fmt.Fprintf(w, "Cluster URL: %s\n", clusterURL)
	fmt.Fprintf(w, "Control Plane URL: %s\n", cpURL)
	fmt.Fprintln(w, "")
}

// collectPodLogs collects logs from all pods in the namespace
func (c *Collector) collectPodLogs(namespace, logDir string, scriptLog io.Writer) error {
	logsSubDir := filepath.Join(logDir, "logs")
	if err := os.MkdirAll(logsSubDir, 0755); err != nil {
		return err
	}

	fmt.Printf("  📋 Collecting pod information for namespace: %s\n", namespace)
	fmt.Fprintf(scriptLog, "  Collecting pod information for namespace: %s\n", namespace)

	// Get all pods in namespace
	podsOutput, err := c.runKubectl("get", "pods", "-n", namespace, "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return err
	}

	pods := strings.Fields(strings.TrimSpace(podsOutput))
	if len(pods) == 0 {
		fmt.Printf("  ❌ No pods found in namespace: %s\n", namespace)
		fmt.Fprintf(scriptLog, "  No pods found in namespace: %s\n", namespace)
		return nil
	}

	fmt.Printf("  ✅ Found %d pods in namespace: %s\n", len(pods), namespace)
	fmt.Fprintf(scriptLog, "  Found %d pods in namespace: %s\n", len(pods), namespace)

	for i, pod := range pods {
		fmt.Printf("  🔄 [%d/%d] Processing pod: %s\n", i+1, len(pods), pod)
		fmt.Fprintf(scriptLog, "  Processing pod: %s\n", pod)

		// Get regular containers
		containersOutput, _ := c.runKubectl("get", "pod", pod, "-n", namespace, "-o", "jsonpath={.spec.containers[*].name}")
		containers := strings.Fields(strings.TrimSpace(containersOutput))
		fmt.Printf("    📦 Regular containers found: %d\n", len(containers))
		fmt.Fprintf(scriptLog, "    Regular containers found: %d\n", len(containers))

		// Get init containers
		initContainersOutput, _ := c.runKubectl("get", "pod", pod, "-n", namespace, "-o", "jsonpath={.spec.initContainers[*].name}")
		initContainers := strings.Fields(strings.TrimSpace(initContainersOutput))
		if len(initContainers) > 0 {
			fmt.Printf("    🚀 Init containers found: %d\n", len(initContainers))
		}
		fmt.Fprintf(scriptLog, "    Init containers found: %d\n", len(initContainers))

		// Collect logs for regular containers
		for j, container := range containers {
			logFile := filepath.Join(logsSubDir, fmt.Sprintf("%s_%s.log", pod, container))
			fmt.Printf("    📋 [%d/%d] Collecting logs: %s/%s\n", j+1, len(containers), pod, container)
			fmt.Fprintf(scriptLog, "    Collecting logs for Pod: %s, Container: %s\n", pod, container)

			if err := c.collectContainerLogs(pod, container, namespace, logFile, false); err != nil {
				fmt.Printf("      ⚠️  Warning: Failed to collect logs for container: %s\n", container)
				fmt.Fprintf(scriptLog, "      ⚠ Warning: Failed to collect logs for container: %s\n", container)
			} else {
				fmt.Printf("      ✅ Logs saved\n")
				fmt.Fprintf(scriptLog, "      ✓ Logs saved to: %s\n", logFile)
			}
		}

		// Collect logs for init containers
		for j, container := range initContainers {
			logFile := filepath.Join(logsSubDir, fmt.Sprintf("%s_%s_init.log", pod, container))
			fmt.Printf("    🚀 [%d/%d] Collecting init logs: %s/%s\n", j+1, len(initContainers), pod, container)
			fmt.Fprintf(scriptLog, "    Collecting logs for Pod: %s, Init Container: %s\n", pod, container)

			if err := c.collectContainerLogs(pod, container, namespace, logFile, true); err != nil {
				fmt.Printf("      ⚠️  Warning: Failed to collect logs for init container: %s\n", container)
				fmt.Fprintf(scriptLog, "      ⚠ Warning: Failed to collect logs for init container: %s\n", container)
			} else {
				fmt.Printf("      ✅ Init logs saved\n")
				fmt.Fprintf(scriptLog, "      ✓ Init container logs saved to: %s\n", logFile)
			}
		}
	}

	return nil
}

// collectContainerLogs collects logs from a specific container
func (c *Collector) collectContainerLogs(pod, container, namespace, logFile string, isInit bool) error {
	args := []string{"logs", "--timestamps", pod, "-c", container, "-n", namespace}

	output, err := c.runKubectl(args...)
	if err != nil {
		return err
	}

	return os.WriteFile(logFile, []byte(output), 0644)
}

// collectAdditionalInfo collects namespace-specific additional information
func (c *Collector) collectAdditionalInfo(namespace, logDir string, scriptLog io.Writer) error {
	switch namespace {
	case "runai":
		return c.collectRunaiInfo(logDir, scriptLog)
	case "runai-backend":
		return c.collectBackendInfo(logDir, scriptLog)
	}
	return nil
}

// collectRunaiInfo collects information specific to the runai namespace
func (c *Collector) collectRunaiInfo(logDir string, scriptLog io.Writer) error {
	actions := []struct {
		name     string
		filename string
		cmd      func() (string, error)
	}{
		{"Helm charts list", "helm_charts_list.txt", func() (string, error) {
			return c.runHelm("ls", "-A")
		}},
		{"Helm values for runai-cluster", "helm-values_runai-cluster.yaml", func() (string, error) {
			return c.runHelm("-n", "runai", "get", "values", "runai-cluster")
		}},
		{"ConfigMap runai-public", "cm_runai-public.yaml", func() (string, error) {
			return c.runKubectl("-n", "runai", "get", "cm", "runai-public", "-o", "yaml")
		}},
		{"Pod list for runai namespace", "pod-list_runai.txt", func() (string, error) {
			return c.runKubectl("-n", "runai", "get", "pods", "-o", "wide")
		}},
		{"Node list", "node-list.txt", func() (string, error) {
			return c.runKubectl("get", "nodes", "-o", "wide")
		}},
		{"RunAI config", "runaiconfig.yaml", func() (string, error) {
			return c.runKubectl("-n", "runai", "get", "runaiconfig", "runai", "-o", "yaml")
		}},
		{"Engine config", "engine-config.yaml", func() (string, error) {
			return c.runKubectl("-n", "runai", "get", "configs.engine.run.ai", "engine-config", "-o", "yaml")
		}},
	}

	for i, action := range actions {
		fmt.Printf("  📊 [%d/%d] Collecting %s...\n", i+1, len(actions), action.name)
		fmt.Fprintf(scriptLog, "Collecting %s...\n", action.name)
		output, err := action.cmd()
		if err != nil {
			fmt.Printf("    ⚠️  Warning: Failed to collect %s: %v\n", action.name, err)
			fmt.Fprintf(scriptLog, "  ⚠ Warning: Failed to collect %s: %v\n", action.name, err)
			continue
		}

		filePath := filepath.Join(logDir, action.filename)
		if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
			fmt.Printf("    ⚠️  Warning: Failed to write %s: %v\n", action.filename, err)
			fmt.Fprintf(scriptLog, "  ⚠ Warning: Failed to write %s: %v\n", action.filename, err)
			continue
		}

		fmt.Printf("    ✅ %s saved\n", action.name)
		fmt.Fprintf(scriptLog, "  ✓ %s saved\n", action.name)
	}

	return nil
}

// collectBackendInfo collects information specific to the runai-backend namespace
func (c *Collector) collectBackendInfo(logDir string, scriptLog io.Writer) error {
	actions := []struct {
		name     string
		filename string
		cmd      func() (string, error)
	}{
		{"Pod list for runai-backend namespace", "pod-list_runai-backend.txt", func() (string, error) {
			return c.runKubectl("-n", "runai-backend", "get", "pods", "-o", "wide")
		}},
		{"Helm values for runai-backend", "helm-values_runai-backend.yaml", func() (string, error) {
			return c.runHelm("-n", "runai-backend", "get", "values", "runai-backend")
		}},
	}

	for i, action := range actions {
		fmt.Printf("  📊 [%d/%d] Collecting %s...\n", i+1, len(actions), action.name)
		fmt.Fprintf(scriptLog, "Collecting %s...\n", action.name)
		output, err := action.cmd()
		if err != nil {
			fmt.Printf("    ⚠️  Warning: Failed to collect %s: %v\n", action.name, err)
			fmt.Fprintf(scriptLog, "  ⚠ Warning: Failed to collect %s: %v\n", action.name, err)
			continue
		}

		filePath := filepath.Join(logDir, action.filename)
		if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
			fmt.Printf("    ⚠️  Warning: Failed to write %s: %v\n", action.filename, err)
			fmt.Fprintf(scriptLog, "  ⚠ Warning: Failed to write %s: %v\n", action.filename, err)
			continue
		}

		fmt.Printf("    ✅ %s saved\n", action.name)
		fmt.Fprintf(scriptLog, "  ✓ %s saved\n", action.name)
	}

	return nil
}

// createArchive creates a tar.gz archive of the log directory
func (c *Collector) createArchive(logDir, archiveName string, scriptLog io.Writer) error {
	fmt.Printf("  📦 Creating archive %s...\n", archiveName)
	fmt.Fprintf(scriptLog, "Creating tar archive...\n")

	// Create the archive file
	archiveFile, err := os.Create(archiveName)
	if err != nil {
		return err
	}
	defer archiveFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk the directory and add files to archive
	err = filepath.Walk(logDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// Update the name to maintain directory structure
		header.Name = file

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write the content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			_, err = io.Copy(tarWriter, data)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Get archive info
	archiveInfo, err := os.Stat(archiveName)
	if err == nil {
		fmt.Printf("  ✅ Archive created: %s (%.2f MB)\n", archiveName, float64(archiveInfo.Size())/1024/1024)
		fmt.Fprintf(scriptLog, "  ✓ Archive created\n")
		fmt.Fprintf(scriptLog, "Archive details: %s (%d bytes)\n", archiveName, archiveInfo.Size())
	}

	fmt.Printf("  🧹 Cleaning up temporary directory...\n")
	fmt.Fprintf(scriptLog, "Cleaning up temporary directory...\n")
	fmt.Fprintf(scriptLog, "  ✓ Temporary directory will be removed\n")
	fmt.Fprintf(scriptLog, "=== Log Collection Completed at %s ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(scriptLog, "Logs and info archived to %s\n", archiveName)

	return nil
}

// runKubectl executes kubectl command and returns output
func (c *Collector) runKubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// runHelm executes helm command and returns output
func (c *Collector) runHelm(args ...string) (string, error) {
	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// runCommand executes a generic command and returns output
func (c *Collector) runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// CollectWorkloadInfo collects detailed information about a specific RunAI workload
func (c *Collector) CollectWorkloadInfo(project, workloadType, name string) error {
	fmt.Printf("🚀 Starting workload info collection for '%s' (%s) in project '%s'...\n", name, workloadType, project)

	// Check required tools
	if err := c.checkRequiredTools(); err != nil {
		return fmt.Errorf("required tools check failed: %w", err)
	}

	// Map type aliases to canonical resource names
	canonicalType := c.getCanonicalWorkloadType(workloadType)
	if canonicalType == "" {
		return fmt.Errorf("invalid workload type: %s. Valid types: tw, iw, infw, dw, dinfw, ew", workloadType)
	}

	// Resolve namespace from project
	fmt.Printf("🔍 Resolving namespace for project '%s'...\n", project)
	namespace, err := c.runKubectl("get", "ns", "-l", fmt.Sprintf("runai/queue=%s", project), "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil || strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("no namespace found for project: %s", project)
	}
	namespace = strings.TrimSpace(namespace)
	fmt.Printf("✅ Found namespace: %s\n", namespace)

	// Create timestamp and prepare file names
	timestamp := time.Now().Format("2006_01_02-15_04")
	typeSafe := strings.Replace(workloadType, "/", "_", -1)
	archiveName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", project, typeSafe, name, timestamp)

	var outputFiles []string

	fmt.Println("\n📁 Starting collection process...")

	// Collect workload YAML
	if file, err := c.getWorkloadYAML(namespace, name, canonicalType, typeSafe); err != nil {
		fmt.Printf("❌ Failed to get workload YAML: %v\n", err)
	} else {
		outputFiles = append(outputFiles, file)
	}

	// Collect RunAIJob YAML
	if file, err := c.getRunAIJobYAML(namespace, name, typeSafe); err != nil {
		fmt.Printf("❌ Failed to get RunAIJob YAML: %v\n", err)
	} else {
		outputFiles = append(outputFiles, file)
	}

	// Collect Pod YAML
	if file, err := c.getPodYAML(namespace, name, typeSafe); err != nil {
		fmt.Printf("❌ Failed to get Pod YAML: %v\n", err)
	} else {
		outputFiles = append(outputFiles, file)
	}

	// Collect PodGroup YAML
	if file, err := c.getPodGroupYAML(namespace, name, typeSafe); err != nil {
		fmt.Printf("❌ Failed to get PodGroup YAML: %v\n", err)
	} else {
		outputFiles = append(outputFiles, file)
	}

	// Collect Pod logs
	if files, err := c.getPodLogs(namespace, name, typeSafe); err != nil {
		fmt.Printf("❌ Failed to get Pod logs: %v\n", err)
	} else {
		outputFiles = append(outputFiles, files...)
	}

	// Collect KSVC for inference workloads
	if canonicalType == "inferenceworkloads" {
		if file, err := c.getKSVCYAML(namespace, name, typeSafe); err != nil {
			fmt.Printf("❌ Failed to get KSVC YAML: %v\n", err)
		} else {
			outputFiles = append(outputFiles, file)
		}
	}

	// Create archive
	fmt.Printf("\n📦 Creating archive: %s\n", archiveName)
	if err := c.createWorkloadArchive(archiveName, outputFiles); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Clean up individual files
	fmt.Println("\n🧹 Cleaning up individual files...")
	for _, file := range outputFiles {
		if err := os.Remove(file); err == nil {
			fmt.Printf("  🗑️  Deleted: %s\n", file)
		}
	}

	fmt.Printf("\n✅ Workload info collection completed!\n")
	fmt.Printf("📦 Archive created: %s\n", archiveName)

	return nil
}

// CollectSchedulerInfo collects RunAI scheduler information and resources
func (c *Collector) CollectSchedulerInfo() error {
	fmt.Println("🚀 Starting RunAI scheduler info collection...")

	// Check required tools
	if err := c.checkRequiredTools(); err != nil {
		return fmt.Errorf("required tools check failed: %w", err)
	}

	// Test cluster connectivity
	if _, err := c.runKubectl("cluster-info"); err != nil {
		return fmt.Errorf("cannot connect to Kubernetes cluster: %w", err)
	}
	fmt.Println("✅ Connected to Kubernetes cluster")

	// Create timestamp and archive name
	timestamp := time.Now().Format("02-01-2006_15-04")
	archiveName := fmt.Sprintf("scheduler_info_dump_%s", timestamp)
	tempDir := archiveName

	fmt.Printf("📁 Creating temp directory: %s\n", tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		return fmt.Errorf("failed to change to temp directory: %w", err)
	}

	// Ensure we change back to original directory
	defer func() {
		os.Chdir(originalDir)
	}()

	// Collect scheduler resources
	resources := []struct {
		resourceType string
		singular     string
	}{
		{"projects", "project"},
		{"queues", "queue"},
		{"nodepools", "nodepool"},
		{"departments", "department"},
	}

	for _, resource := range resources {
		if err := c.dumpSchedulerResource(resource.resourceType, resource.singular); err != nil {
			fmt.Printf("⚠️  Warning: Failed to dump %s: %v\n", resource.resourceType, err)
		}
	}

	// Go back to original directory
	if err := os.Chdir(originalDir); err != nil {
		return fmt.Errorf("failed to change back to original directory: %w", err)
	}

	// Create archive
	archiveFile := fmt.Sprintf("%s.tar.gz", archiveName)
	fmt.Printf("\n📦 Creating archive: %s\n", archiveFile)

	cmd := fmt.Sprintf("tar -czf %s %s", archiveFile, tempDir)
	if _, err := c.runCommand("sh", "-c", cmd); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Clean up temp directory
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Printf("⚠️  Warning: Failed to clean up temp directory: %v\n", err)
	}

	fmt.Printf("\n✅ Scheduler info collection completed!\n")
	fmt.Printf("📦 Archive created: %s\n", archiveFile)
	fmt.Println("\n📋 Archive contains:")
	fmt.Println("  - projects_list.txt (projects list)")
	fmt.Println("  - project_*.yaml (individual projects)")
	fmt.Println("  - queues_list.txt (queues list)")
	fmt.Println("  - queue_*.yaml (individual queues)")
	fmt.Println("  - nodepools_list.txt (nodepools list)")
	fmt.Println("  - nodepool_*.yaml (individual nodepools)")
	fmt.Println("  - departments_list.txt (departments list)")
	fmt.Println("  - department_*.yaml (individual departments)")

	return nil
}

// RunTests performs environment verification and connectivity tests
func (c *Collector) RunTests() error {
	fmt.Println("🧪 Running environment tests for RunAI log collection...")
	fmt.Println()

	// Test 1: Check required tools
	fmt.Println("🔧 Testing required tools...")
	if err := c.testRequiredTools(); err != nil {
		return err
	}

	// Test 2: Test cluster connectivity
	fmt.Println("\n🌐 Testing cluster connectivity...")
	if err := c.testClusterConnectivity(); err != nil {
		return err
	}

	// Test 3: Check RunAI namespaces
	fmt.Println("\n📋 Checking RunAI namespaces...")
	if err := c.testRunAINamespaces(); err != nil {
		return err
	}

	// Test 4: Extract and display RunAI information
	fmt.Println("\n📊 Retrieving RunAI cluster information...")
	if err := c.displayRunAIInfo(); err != nil {
		fmt.Printf("⚠️  Warning: Could not retrieve RunAI information: %v\n", err)
	}

	fmt.Println("\n🎉 All tests passed! Environment is ready for log collection.")
	fmt.Println("\nRun 'nmcrun logs' to start collecting logs.")

	return nil
}

// testRequiredTools checks if kubectl and helm are available
func (c *Collector) testRequiredTools() error {
	tools := []string{"kubectl", "helm"}

	for _, tool := range tools {
		fmt.Printf("  🔍 Checking %s... ", tool)
		if _, err := exec.LookPath(tool); err != nil {
			fmt.Printf("❌ NOT FOUND\n")
			return fmt.Errorf("'%s' command not found. Please install %s and ensure it's in your PATH", tool, tool)
		}

		// Get version for additional verification
		var versionCmd []string
		switch tool {
		case "kubectl":
			versionCmd = []string{"version", "--client"}
		case "helm":
			versionCmd = []string{"version", "--short"}
		}

		output, err := exec.Command(tool, versionCmd...).CombinedOutput()
		if err != nil {
			// For kubectl, try alternative version command
			if tool == "kubectl" {
				output, err = exec.Command("kubectl", "version", "--client=true", "--output=yaml").CombinedOutput()
				if err != nil {
					fmt.Printf("⚠️  FOUND (version check failed)\n")
					fmt.Printf("    Warning: %s found but version check failed: %v\n", tool, err)
					continue
				}
			} else {
				fmt.Printf("❌ INVALID\n")
				return fmt.Errorf("'%s' found but not working properly: %v", tool, err)
			}
		}

		version := strings.TrimSpace(string(output))
		// Extract just the version line for kubectl
		if tool == "kubectl" {
			lines := strings.Split(version, "\n")
			for _, line := range lines {
				if strings.Contains(line, "Client Version") || strings.Contains(line, "gitVersion") {
					version = strings.TrimSpace(line)
					break
				}
			}
		}

		if len(version) > 60 {
			version = version[:60] + "..."
		}
		fmt.Printf("✅ %s\n", version)
	}

	return nil
}

// testClusterConnectivity tests if kubectl can connect to the cluster
func (c *Collector) testClusterConnectivity() error {
	fmt.Printf("  🔗 Testing kubectl cluster connection... ")

	// Try to get cluster info
	output, err := c.runKubectl("cluster-info")
	if err != nil {
		fmt.Printf("❌ FAILED\n")
		return fmt.Errorf("kubectl cannot connect to cluster: %v", err)
	}

	fmt.Printf("✅ CONNECTED\n")

	// Try to get nodes to verify permissions
	fmt.Printf("  👥 Testing cluster permissions... ")
	_, err = c.runKubectl("get", "nodes", "--no-headers")
	if err != nil {
		fmt.Printf("⚠️  LIMITED\n")
		fmt.Printf("    Warning: Cannot list nodes (may have limited permissions): %v\n", err)
	} else {
		fmt.Printf("✅ SUFFICIENT\n")
	}

	// Show current context
	context, err := c.runKubectl("config", "current-context")
	if err == nil {
		fmt.Printf("  📍 Current context: %s\n", strings.TrimSpace(context))
	}

	// Show cluster info excerpt
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Kubernetes control plane") || strings.Contains(line, "Kubernetes master") {
			fmt.Printf("  🎯 %s\n", strings.TrimSpace(line))
			break
		}
	}

	return nil
}

// testRunAINamespaces checks if RunAI namespaces exist
func (c *Collector) testRunAINamespaces() error {
	namespaces := []string{"runai", "runai-backend"}
	foundNamespaces := []string{}

	for _, namespace := range namespaces {
		fmt.Printf("  📂 Checking namespace '%s'... ", namespace)

		if c.namespaceExists(namespace) {
			fmt.Printf("✅ EXISTS\n")
			foundNamespaces = append(foundNamespaces, namespace)

			// Count pods in namespace
			podsOutput, err := c.runKubectl("get", "pods", "-n", namespace, "--no-headers")
			if err == nil {
				podLines := strings.Split(strings.TrimSpace(podsOutput), "\n")
				if len(podLines) == 1 && podLines[0] == "" {
					fmt.Printf("    📦 0 pods found\n")
				} else {
					fmt.Printf("    📦 %d pods found\n", len(podLines))
				}
			}
		} else {
			fmt.Printf("❌ NOT FOUND\n")
		}
	}

	if len(foundNamespaces) == 0 {
		return fmt.Errorf("no RunAI namespaces found. Expected 'runai' and/or 'runai-backend'")
	}

	fmt.Printf("  ✅ Found %d RunAI namespace(s): %s\n", len(foundNamespaces), strings.Join(foundNamespaces, ", "))
	return nil
}

// displayRunAIInfo extracts and displays RunAI cluster information
func (c *Collector) displayRunAIInfo() error {
	// Check if runai namespace exists
	if !c.namespaceExists("runai") {
		return fmt.Errorf("runai namespace not found")
	}

	fmt.Printf("  🔍 Extracting RunAI configuration...\n")

	// Extract cluster and control plane URLs
	clusterURL, cpURL, err := c.extractClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to extract cluster info: %w", err)
	}

	fmt.Printf("  🌐 Cluster URL: %s\n", clusterURL)
	fmt.Printf("  🎛️  Control Plane URL: %s\n", cpURL)

	// Try to get RunAI version
	fmt.Printf("  📊 Checking RunAI components...\n")

	// Check if runaiconfig exists
	configOutput, err := c.runKubectl("-n", "runai", "get", "runaiconfig", "runai", "-o", "jsonpath={.metadata.name}")
	if err == nil && strings.TrimSpace(configOutput) == "runai" {
		fmt.Printf("    ✅ RunAI configuration found\n")

		// Try to get RunAI version from config
		version, err := c.runKubectl("-n", "runai", "get", "runaiconfig", "runai", "-o", "jsonpath={.spec.global.image.tag}")
		if err == nil && strings.TrimSpace(version) != "" {
			fmt.Printf("    📋 RunAI version: %s\n", strings.TrimSpace(version))
		}
	} else {
		fmt.Printf("    ⚠️  RunAI configuration not found\n")
	}

	// Get RunAI cluster version from configmap
	clusterVersion, err := c.runKubectl("-n", "runai", "get", "cm", "runai-public", "-o", "jsonpath={.data.cluster-version}")
	if err == nil && strings.TrimSpace(clusterVersion) != "" {
		fmt.Printf("    📊 RunAI cluster version: %s\n", strings.TrimSpace(clusterVersion))
	} else {
		fmt.Printf("    ⚠️  RunAI cluster version not found\n")
	}

	// Check Helm charts
	helmOutput, err := c.runHelm("ls", "-n", "runai", "--no-headers")
	if err == nil {
		helmLines := strings.Split(strings.TrimSpace(helmOutput), "\n")
		if len(helmLines) > 0 && helmLines[0] != "" {
			fmt.Printf("    ✅ %d Helm chart(s) found in runai namespace\n", len(helmLines))
			for _, line := range helmLines {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					fmt.Printf("      - %s (%s)\n", fields[0], fields[1])
				}
			}
		}
	}

	return nil
}

// Helper methods for workload collection

// getCanonicalWorkloadType maps type aliases to canonical k8s resource names
func (c *Collector) getCanonicalWorkloadType(workloadType string) string {
	switch workloadType {
	case "dinfw", "distributedinferenceworkloads":
		return "distributedinferenceworkloads"
	case "dw", "distributedworkloads":
		return "distributedworkloads"
	case "ew", "externalworkloads":
		return "externalworkloads"
	case "infw", "inferenceworkloads":
		return "inferenceworkloads"
	case "iw", "interactiveworkloads":
		return "interactiveworkloads"
	case "tw", "trainingworkloads":
		return "trainingworkloads"
	default:
		return ""
	}
}

// getWorkloadYAML retrieves workload YAML
func (c *Collector) getWorkloadYAML(namespace, workload, canonicalType, typeSafe string) (string, error) {
	filename := fmt.Sprintf("%s_%s_workload.yaml", workload, typeSafe)
	fmt.Printf("  📄 Getting %s YAML...\n", canonicalType)

	output, err := c.runKubectl("-n", namespace, "get", canonicalType, workload, "-o", "yaml")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return "", err
	}

	fmt.Printf("    ✅ Workload YAML retrieved\n")
	return filename, nil
}

// getRunAIJobYAML retrieves RunAIJob YAML
func (c *Collector) getRunAIJobYAML(namespace, workload, typeSafe string) (string, error) {
	filename := fmt.Sprintf("%s_%s_runaijob.yaml", workload, typeSafe)
	fmt.Printf("  📄 Getting RunAIJob YAML...\n")

	output, err := c.runKubectl("-n", namespace, "get", "rj", workload, "-o", "yaml")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return "", err
	}

	fmt.Printf("    ✅ RunAIJob YAML retrieved\n")
	return filename, nil
}

// getPodYAML retrieves pod YAML
func (c *Collector) getPodYAML(namespace, workload, typeSafe string) (string, error) {
	filename := fmt.Sprintf("%s_%s_pod.yaml", workload, typeSafe)
	fmt.Printf("  📄 Getting Pod YAML...\n")

	output, err := c.runKubectl("-n", namespace, "get", "pod", "-l", fmt.Sprintf("workloadName=%s", workload), "-o", "yaml")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return "", err
	}

	fmt.Printf("    ✅ Pod YAML retrieved\n")
	return filename, nil
}

// getPodGroupYAML retrieves podgroup YAML
func (c *Collector) getPodGroupYAML(namespace, workload, typeSafe string) (string, error) {
	filename := fmt.Sprintf("%s_%s_podgroup.yaml", workload, typeSafe)
	fmt.Printf("  📄 Getting PodGroup YAML...\n")

	output, err := c.runKubectl("-n", namespace, "get", "pg", "-l", fmt.Sprintf("workloadName=%s", workload), "-o", "yaml")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return "", err
	}

	fmt.Printf("    ✅ PodGroup YAML retrieved\n")
	return filename, nil
}

// getPodLogs retrieves pod logs
func (c *Collector) getPodLogs(namespace, workload, typeSafe string) ([]string, error) {
	fmt.Printf("  📄 Getting Pod Logs...\n")

	// Get all pods for this workload
	podsOutput, err := c.runKubectl("-n", namespace, "get", "pod", "-l", fmt.Sprintf("workloadName=%s", workload), "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, err
	}

	pods := strings.Fields(strings.TrimSpace(podsOutput))
	if len(pods) == 0 {
		fmt.Printf("    ⚠️  No pods found for workload: %s\n", workload)
		return []string{}, nil
	}

	var outputFiles []string

	// Iterate through each pod
	for _, pod := range pods {
		fmt.Printf("    🐳 Processing pod: %s\n", pod)

		// Get all containers for this pod
		containersOutput, err := c.runKubectl("-n", namespace, "get", "pod", pod, "-o", "jsonpath={.spec.initContainers[*].name} {.spec.containers[*].name}")
		if err != nil {
			continue
		}

		containers := strings.Fields(strings.TrimSpace(containersOutput))

		// Iterate through each container
		for _, container := range containers {
			logFile := fmt.Sprintf("%s_%s_pod_logs_%s.log", workload, typeSafe, container)
			fmt.Printf("      📝 Getting logs for container: %s\n", container)

			output, err := c.runKubectl("-n", namespace, "logs", pod, "-c", container)
			if err == nil {
				if err := os.WriteFile(logFile, []byte(output), 0644); err == nil {
					fmt.Printf("        ✅ Container logs retrieved: %s\n", container)
					outputFiles = append(outputFiles, logFile)
				}
			} else {
				fmt.Printf("        ❌ Failed to retrieve logs for container: %s\n", container)
			}
		}
	}

	if len(outputFiles) > 0 {
		fmt.Printf("    ✅ Pod logs retrieved for %d containers\n", len(outputFiles))
	} else {
		fmt.Printf("    ❌ No container logs were successfully retrieved\n")
	}

	return outputFiles, nil
}

// getKSVCYAML retrieves KSVC YAML for inference workloads
func (c *Collector) getKSVCYAML(namespace, workload, typeSafe string) (string, error) {
	filename := fmt.Sprintf("%s_%s_ksvc.yaml", workload, typeSafe)
	fmt.Printf("  📄 Getting KSVC YAML...\n")

	output, err := c.runKubectl("-n", namespace, "get", "ksvc", workload, "-o", "yaml")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return "", err
	}

	fmt.Printf("    ✅ KSVC YAML retrieved\n")
	return filename, nil
}

// createWorkloadArchive creates an archive from collected files
func (c *Collector) createWorkloadArchive(archiveName string, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to archive")
	}

	// Create tar.gz archive
	archiveFile, err := os.Create(archiveName)
	if err != nil {
		return err
	}
	defer archiveFile.Close()

	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, file := range files {
		if err := c.addFileToTar(tarWriter, file); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", file, err)
		}
	}

	return nil
}

// addFileToTar adds a file to tar archive
func (c *Collector) addFileToTar(tarWriter *tar.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	header.Name = filename

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}

// dumpSchedulerResource dumps a scheduler resource type
func (c *Collector) dumpSchedulerResource(resourceType, singular string) error {
	fmt.Printf("📊 Dumping %s...\n", resourceType)

	// Get resource list
	listFile := fmt.Sprintf("%s_list.txt", resourceType)
	output, err := c.runKubectl("get", resourceType)
	if err != nil {
		return fmt.Errorf("failed to get %s list: %w", resourceType, err)
	}

	if err := os.WriteFile(listFile, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write %s list: %w", resourceType, err)
	}

	fmt.Printf("✅ %s list saved to %s\n", resourceType, listFile)

	// Extract individual manifests
	fmt.Printf("📄 Extracting individual %s manifests...\n", resourceType)

	resourcesOutput, err := c.runKubectl("get", resourceType, "--no-headers", "-o", "custom-columns=:metadata.name")
	if err != nil {
		return fmt.Errorf("failed to get %s names: %w", resourceType, err)
	}

	resources := strings.Fields(strings.TrimSpace(resourcesOutput))
	for _, resource := range resources {
		if resource != "" {
			manifestFile := fmt.Sprintf("%s_%s.yaml", singular, resource)

			manifestOutput, err := c.runKubectl("get", singular, resource, "-o", "yaml")
			if err != nil {
				fmt.Printf("  ⚠️  Failed to get %s %s: %v\n", singular, resource, err)
				continue
			}

			if err := os.WriteFile(manifestFile, []byte(manifestOutput), 0644); err != nil {
				fmt.Printf("  ⚠️  Failed to write %s %s: %v\n", singular, resource, err)
				continue
			}

			fmt.Printf("  ✅ Extracted %s: %s\n", singular, resource)
		}
	}

	return nil
}
