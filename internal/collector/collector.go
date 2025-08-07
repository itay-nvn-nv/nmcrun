package collector

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

type Collector struct {
	namespaces    []string
	logDir        string
	timestamp     string
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
}

// New creates a new collector instance
func New() (*Collector, error) {
	restConfig, err := getKubernetesConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Collector{
		namespaces:    []string{"runai-backend", "runai"},
		timestamp:     time.Now().Format("02-01-2006_15-04"),
		clientset:     clientset,
		dynamicClient: dynamicClient,
		config:        restConfig,
	}, nil
}

// getKubernetesConfig creates a Kubernetes REST config using multiple authentication methods
func getKubernetesConfig() (*rest.Config, error) {
	// Method 1: Try in-cluster config first (for pods running inside the cluster)
	if config, err := rest.InClusterConfig(); err == nil {
		fmt.Printf("🔗 Using in-cluster authentication\n")
		return config, nil
	}

	// Method 2: Try kubeconfig file
	if config, err := tryKubeconfigAuth(); err == nil {
		fmt.Printf("🔗 Using kubeconfig file authentication\n")
		return config, nil
	}

	// Method 3: Try service account token file
	if config, err := tryServiceAccountTokenAuth(); err == nil {
		fmt.Printf("🔗 Using service account token authentication\n")
		return config, nil
	}

	// Method 4: Try environment variables
	if config, err := tryEnvironmentAuth(); err == nil {
		fmt.Printf("🔗 Using environment variable authentication\n")
		return config, nil
	}

	return nil, fmt.Errorf(`no valid Kubernetes authentication method found. Please ensure one of the following:
1. Running inside a Kubernetes cluster with a service account
2. Have a valid kubeconfig file at ~/.kube/config or set KUBECONFIG env var
3. Have KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT environment variables set
4. Have a service account token file available

For more details, see: https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/`)
}

// tryKubeconfigAuth attempts to authenticate using kubeconfig files
func tryKubeconfigAuth() (*rest.Config, error) {
	// Use the default loading rules (checks KUBECONFIG env var, ~/.kube/config, etc.)
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	// Load the config
	config, err := loadingRules.Load()
	if err != nil {
		return nil, err
	}

	// Create client config
	clientConfig := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{})

	// Get REST config
	return clientConfig.ClientConfig()
}

// tryServiceAccountTokenAuth attempts to authenticate using a service account token file
func tryServiceAccountTokenAuth() (*rest.Config, error) {
	const (
		tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
		caCertFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	)

	// Check if service account files exist
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("service account token file not found")
	}

	// Read token
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token: %w", err)
	}

	// Get Kubernetes API server from environment
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("KUBERNETES_SERVICE_HOST or KUBERNETES_SERVICE_PORT environment variables not set")
	}

	// Create config
	config := &rest.Config{
		Host:        fmt.Sprintf("https://%s:%s", host, port),
		BearerToken: string(token),
	}

	// Set CA certificate if available
	if _, err := os.Stat(caCertFile); err == nil {
		config.TLSClientConfig.CAFile = caCertFile
	} else {
		// If no CA file, skip TLS verification (not recommended for production)
		config.TLSClientConfig.Insecure = true
	}

	return config, nil
}

// tryEnvironmentAuth attempts to authenticate using environment variables
func tryEnvironmentAuth() (*rest.Config, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	token := os.Getenv("KUBERNETES_TOKEN")

	if host == "" || port == "" {
		return nil, fmt.Errorf("KUBERNETES_SERVICE_HOST or KUBERNETES_SERVICE_PORT environment variables not set")
	}

	config := &rest.Config{
		Host: fmt.Sprintf("https://%s:%s", host, port),
	}

	// Use token if provided
	if token != "" {
		config.BearerToken = token
	}

	// Check for custom CA certificate path
	if caCertPath := os.Getenv("KUBERNETES_CA_CERT_FILE"); caCertPath != "" {
		config.TLSClientConfig.CAFile = caCertPath
	} else {
		// Skip TLS verification if no CA cert specified (not recommended for production)
		config.TLSClientConfig.Insecure = true
	}

	return config, nil
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
	fmt.Println("🔧 Checking system requirements...")

	// No external tools required! Everything is handled by native Go libraries
	fmt.Println("✅ All requirements satisfied (no external tools needed)")
	return nil
}

// extractClusterInfo gets cluster and control plane URLs
func (c *Collector) extractClusterInfo() (string, string, error) {
	// Get the runaiconfig resource using dynamic client
	gvr := schema.GroupVersionResource{Group: "run.ai", Version: "v1", Resource: "runaiconfigs"}
	obj, err := c.dynamicClient.Resource(gvr).Namespace("runai").Get(context.TODO(), "runai", metav1.GetOptions{})
	if err != nil {
		return "unknown", "unknown", nil
	}

	// Extract cluster URL
	clusterURL := "unknown"
	if url, found, _ := unstructured.NestedString(obj.Object, "spec", "__internal", "global", "clusterURL"); found {
		clusterURL = url
	}

	// Extract control plane URL
	cpURL := "unknown"
	if url, found, _ := unstructured.NestedString(obj.Object, "spec", "__internal", "global", "controlPlane", "url"); found {
		cpURL = url
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

// removed - replaced with client-go version

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
	pods, err := c.getPods(namespace)
	if err != nil {
		return err
	}
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

		// Get containers for this pod
		containers, initContainers, err := c.getPodContainers(namespace, pod)
		if err != nil {
			fmt.Printf("    ⚠️  Warning: Failed to get containers for pod: %s\n", pod)
			fmt.Fprintf(scriptLog, "    Warning: Failed to get containers for pod: %s\n", pod)
			continue
		}
		fmt.Printf("    📦 Regular containers found: %d\n", len(containers))
		fmt.Fprintf(scriptLog, "    Regular containers found: %d\n", len(containers))
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
	output, err := c.getPodLogsForContainer(namespace, pod, container)
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
		{"Helm releases info", "helm_releases_info.txt", func() (string, error) {
			return c.getHelmReleasesInfo()
		}},
		{"ConfigMap runai-public", "cm_runai-public.yaml", func() (string, error) {
			return c.getConfigMap("runai", "runai-public")
		}},
		{"Pod list for runai namespace", "pod-list_runai.txt", func() (string, error) {
			return c.getPodsWide("runai")
		}},
		{"Node list", "node-list.txt", func() (string, error) {
			return c.getNodesWide()
		}},
		{"RunAI config", "runaiconfig.yaml", func() (string, error) {
			return c.getResourceAsYAML("runai", "runaiconfig", "runai")
		}},
		{"Engine config", "engine-config.yaml", func() (string, error) {
			return c.getResourceAsYAML("runai", "configs.engine.run.ai", "engine-config")
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
			return c.getPodsWide("runai-backend")
		}},
		{"Helm releases info (backend)", "helm_releases_info_backend.txt", func() (string, error) {
			return c.getHelmReleasesInfoNamespace("runai-backend")
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

// Helper functions to replace kubectl functionality

// getPods gets pod names in a namespace
func (c *Collector) getPods(namespace string) ([]string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podNames []string
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}
	return podNames, nil
}

// getPodContainers gets container names for a pod
func (c *Collector) getPodContainers(namespace, podName string) ([]string, []string, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	var containers []string
	var initContainers []string

	for _, container := range pod.Spec.Containers {
		containers = append(containers, container.Name)
	}

	for _, container := range pod.Spec.InitContainers {
		initContainers = append(initContainers, container.Name)
	}

	return containers, initContainers, nil
}

// getPodLogs gets logs for a specific container in a pod
func (c *Collector) getPodLogsForContainer(namespace, podName, containerName string) (string, error) {
	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Timestamps: true,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// namespaceExists checks if a namespace exists
func (c *Collector) namespaceExists(namespace string) bool {
	_, err := c.clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	return err == nil
}

// getConfigMap gets a ConfigMap as YAML
func (c *Collector) getConfigMap(namespace, name string) (string, error) {
	cm, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return c.objectToYAML(cm)
}

// getPodsWide gets pods in wide format (similar to kubectl get pods -o wide)
func (c *Collector) getPodsWide(namespace string) (string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString("NAME\tREADY\tSTATUS\tRESTARTS\tAGE\tIP\tNODE\n")

	for _, pod := range pods.Items {
		readyCount := 0
		totalCount := len(pod.Status.ContainerStatuses)
		for _, status := range pod.Status.ContainerStatuses {
			if status.Ready {
				readyCount++
			}
		}

		restarts := int32(0)
		for _, status := range pod.Status.ContainerStatuses {
			restarts += status.RestartCount
		}

		age := time.Since(pod.CreationTimestamp.Time).Truncate(time.Second)

		output.WriteString(fmt.Sprintf("%s\t%d/%d\t%s\t%d\t%s\t%s\t%s\n",
			pod.Name,
			readyCount,
			totalCount,
			pod.Status.Phase,
			restarts,
			age,
			pod.Status.PodIP,
			pod.Spec.NodeName,
		))
	}

	return output.String(), nil
}

// getNodesWide gets nodes in wide format
func (c *Collector) getNodesWide() (string, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString("NAME\tSTATUS\tROLES\tAGE\tVERSION\tINTERNAL-IP\tEXTERNAL-IP\tOS-IMAGE\tKERNEL-VERSION\tCONTAINER-RUNTIME\n")

	for _, node := range nodes.Items {
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				status = "Ready"
				break
			}
		}

		age := time.Since(node.CreationTimestamp.Time).Truncate(time.Second)

		var internalIP, externalIP string
		for _, addr := range node.Status.Addresses {
			switch addr.Type {
			case corev1.NodeInternalIP:
				internalIP = addr.Address
			case corev1.NodeExternalIP:
				externalIP = addr.Address
			}
		}

		output.WriteString(fmt.Sprintf("%s\t%s\t<none>\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			node.Name,
			status,
			age,
			node.Status.NodeInfo.KubeletVersion,
			internalIP,
			externalIP,
			node.Status.NodeInfo.OSImage,
			node.Status.NodeInfo.KernelVersion,
			node.Status.NodeInfo.ContainerRuntimeVersion,
		))
	}

	return output.String(), nil
}

// getResourceAsYAML gets any Kubernetes resource as YAML using dynamic client
func (c *Collector) getResourceAsYAML(namespace, resource, name string) (string, error) {
	// Map common resource types to their GVR with fallback versions
	gvrCandidates := map[string][]schema.GroupVersionResource{
		"runaiconfig":           {{Group: "run.ai", Version: "v1", Resource: "runaiconfigs"}},
		"configs.engine.run.ai": {{Group: "engine.run.ai", Version: "v1", Resource: "configs"}},
		"rj":                    {{Group: "run.ai", Version: "v1", Resource: "runaijobs"}},
		"pg":                    {{Group: "scheduling.run.ai", Version: "v1", Resource: "podgroups"}, {Group: "scheduling.k8s.io", Version: "v1", Resource: "podgroups"}},
		"ksvc":                  {{Group: "serving.knative.dev", Version: "v1", Resource: "services"}},
		// RunAI workload types with multiple version fallbacks
		"trainingworkloads":             {{Group: "run.ai", Version: "v1", Resource: "trainingworkloads"}, {Group: "run.ai", Version: "v2alpha1", Resource: "trainingworkloads"}},
		"interactiveworkloads":          {{Group: "run.ai", Version: "v1", Resource: "interactiveworkloads"}, {Group: "run.ai", Version: "v2alpha1", Resource: "interactiveworkloads"}},
		"inferenceworkloads":            {{Group: "run.ai", Version: "v1", Resource: "inferenceworkloads"}, {Group: "run.ai", Version: "v2alpha1", Resource: "inferenceworkloads"}},
		"distributedworkloads":          {{Group: "run.ai", Version: "v1", Resource: "distributedworkloads"}, {Group: "run.ai", Version: "v2alpha1", Resource: "distributedworkloads"}},
		"distributedinferenceworkloads": {{Group: "run.ai", Version: "v1", Resource: "distributedinferenceworkloads"}, {Group: "run.ai", Version: "v2alpha1", Resource: "distributedinferenceworkloads"}},
		"externalworkloads":             {{Group: "run.ai", Version: "v1", Resource: "externalworkloads"}, {Group: "run.ai", Version: "v2alpha1", Resource: "externalworkloads"}},
		// RunAI scheduler resources
		"projects":    {{Group: "run.ai", Version: "v2", Resource: "projects"}},
		"queues":      {{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}},
		"nodepools":   {{Group: "run.ai", Version: "v1alpha1", Resource: "nodepools"}},
		"departments": {{Group: "scheduling.run.ai", Version: "v1", Resource: "departments"}},
	}

	gvrList, exists := gvrCandidates[resource]
	if !exists {
		return "", fmt.Errorf("unknown resource type: %s", resource)
	}

	var obj runtime.Object
	var lastErr error

	// Try each GVR version until one works
	for _, gvr := range gvrList {
		var err error
		if namespace != "" {
			obj, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		} else {
			obj, err = c.dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
		}

		if err == nil {
			// Success - convert to YAML and return
			return c.objectToYAML(obj)
		}
		lastErr = err
	}

	// If we get here, all GVR versions failed
	return "", lastErr
}

// getPodsWithLabels gets pods with specific label selector
func (c *Collector) getPodsWithLabels(namespace, labelSelector string) (*corev1.PodList, error) {
	return c.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// getPodGroupsWithLabels gets podgroups with specific label selector using dynamic client
func (c *Collector) getPodGroupsWithLabels(namespace, labelSelector string) (*unstructured.UnstructuredList, error) {
	// Try RunAI's custom API group first
	gvr := schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v1", Resource: "podgroups"}

	podGroups, err := c.dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	if err == nil {
		return podGroups, nil
	}

	// Fallback to standard Kubernetes API group
	gvr = schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "podgroups"}

	return c.dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// objectToYAML converts a Kubernetes object to YAML string
func (c *Collector) objectToYAML(obj runtime.Object) (string, error) {
	yamlData, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(yamlData), nil
}

// getNamespaceByLabel gets namespace by label selector
func (c *Collector) getNamespaceByLabel(labelSelector string) (string, error) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", err
	}

	if len(namespaces.Items) == 0 {
		return "", fmt.Errorf("no namespace found with label: %s", labelSelector)
	}

	return namespaces.Items[0].Name, nil
}

// getCurrentContext gets the current kubectl context
func (c *Collector) getCurrentContext() (string, error) {
	// Load kubeconfig to get current context
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

// testClusterConnection tests if we can connect to the cluster
func (c *Collector) testClusterConnection() error {
	_, err := c.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{Limit: 1})
	return err
}

// getHelmReleasesInfo gets Helm release information using Kubernetes API
func (c *Collector) getHelmReleasesInfo() (string, error) {
	// Get Helm releases from secrets in all namespaces
	return c.getHelmReleasesFromSecrets("")
}

// getHelmReleasesInfoNamespace gets Helm release information for a specific namespace
func (c *Collector) getHelmReleasesInfoNamespace(namespace string) (string, error) {
	return c.getHelmReleasesFromSecrets(namespace)
}

// getHelmReleasesFromSecrets extracts Helm release information from Kubernetes secrets
func (c *Collector) getHelmReleasesFromSecrets(namespace string) (string, error) {
	var output strings.Builder
	output.WriteString("# Helm releases information (extracted from Kubernetes secrets)\n")
	output.WriteString("# This replaces 'helm ls' command using native Kubernetes API\n\n")

	// List secrets with Helm-related labels
	var secrets *corev1.SecretList
	var err error

	if namespace != "" {
		secrets, err = c.clientset.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "owner=helm",
		})
	} else {
		secrets, err = c.clientset.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: "owner=helm",
		})
	}

	if err != nil {
		return "", fmt.Errorf("failed to list Helm secrets: %w", err)
	}

	if len(secrets.Items) == 0 {
		output.WriteString("No Helm releases found\n")
		return output.String(), nil
	}

	output.WriteString("NAMESPACE\tNAME\tREVISION\tSTATUS\tCHART\tAPP VERSION\n")

	for _, secret := range secrets.Items {
		// Parse Helm secret
		name := secret.Labels["name"]
		if name == "" {
			continue
		}

		revision := "unknown"
		if rev, exists := secret.Labels["version"]; exists {
			revision = rev
		}

		status := "unknown"
		if stat, exists := secret.Labels["status"]; exists {
			status = stat
		}

		chart := "unknown"
		appVersion := "unknown"

		// Try to extract more info from secret data if available
		if secret.Type == "helm.sh/release.v1" && len(secret.Data) > 0 {
			// For now, just use the labels we have
			// Full parsing would require decoding the Helm release data
		}

		output.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\n",
			secret.Namespace, name, revision, status, chart, appVersion))
	}

	return output.String(), nil
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
	namespace, err := c.getNamespaceByLabel(fmt.Sprintf("runai/queue=%s", project))
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
		if strings.Contains(err.Error(), "unknown resource type") {
			fmt.Printf("❌ Failed to get workload YAML: %v (check if RunAI workload CRDs are installed)\n", err)
		} else {
			fmt.Printf("❌ Failed to get workload YAML: %v\n", err)
		}
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
	if err := c.testClusterConnection(); err != nil {
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
		} else {
			// Validate that the list file has meaningful content
			listFile := fmt.Sprintf("%s_list.txt", resource.resourceType)
			if err := c.validateFileContent(listFile); err != nil {
				fmt.Printf("⚠️  Warning: %v\n", err)
			}
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

// testRequiredTools checks system requirements (no external tools needed)
func (c *Collector) testRequiredTools() error {
	fmt.Printf("  🔧 Checking system requirements... ")

	// No external tools required! Everything uses native Kubernetes Go client libraries
	fmt.Printf("✅ SATISFIED\n")
	fmt.Printf("    Using native Kubernetes Go client libraries (no external tools required)\n")

	return nil
}

// testClusterConnectivity tests if kubectl can connect to the cluster
func (c *Collector) testClusterConnectivity() error {
	fmt.Printf("  🔗 Testing Kubernetes cluster connection... ")

	// Try to get nodes to test connection
	err := c.testClusterConnection()
	if err != nil {
		fmt.Printf("❌ FAILED\n")
		return fmt.Errorf("cannot connect to cluster: %v", err)
	}

	fmt.Printf("✅ CONNECTED\n")

	// Try to get nodes to verify permissions
	fmt.Printf("  👥 Testing cluster permissions... ")
	_, err = c.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{Limit: 1})
	if err != nil {
		fmt.Printf("⚠️  LIMITED\n")
		fmt.Printf("    Warning: Cannot list nodes (may have limited permissions): %v\n", err)
	} else {
		fmt.Printf("✅ SUFFICIENT\n")
	}

	// Show current context
	context, err := c.getCurrentContext()
	if err == nil {
		fmt.Printf("  📍 Current context: %s\n", strings.TrimSpace(context))
	}

	// Show cluster version info
	version, err := c.clientset.Discovery().ServerVersion()
	if err == nil {
		fmt.Printf("  🎯 Kubernetes server version: %s\n", version.String())
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
			pods, err := c.getPods(namespace)
			if err == nil {
				fmt.Printf("    📦 %d pods found\n", len(pods))
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
	gvr := schema.GroupVersionResource{Group: "run.ai", Version: "v1", Resource: "runaiconfigs"}
	runaiConfigObj, err := c.dynamicClient.Resource(gvr).Namespace("runai").Get(context.TODO(), "runai", metav1.GetOptions{})
	if err == nil {
		fmt.Printf("    ✅ RunAI configuration found\n")

		// Try to get RunAI version from config
		if version, found, _ := unstructured.NestedString(runaiConfigObj.Object, "spec", "global", "image", "tag"); found && strings.TrimSpace(version) != "" {
			fmt.Printf("    📋 RunAI version: %s\n", strings.TrimSpace(version))
		}
	} else {
		fmt.Printf("    ⚠️  RunAI configuration not found\n")
	}

	// Get RunAI cluster version from configmap
	cm, err := c.clientset.CoreV1().ConfigMaps("runai").Get(context.TODO(), "runai-public", metav1.GetOptions{})
	if err == nil {
		if clusterVersion, exists := cm.Data["cluster-version"]; exists && strings.TrimSpace(clusterVersion) != "" {
			fmt.Printf("    📊 RunAI cluster version: %s\n", strings.TrimSpace(clusterVersion))
		} else {
			fmt.Printf("    ⚠️  RunAI cluster version not found in configmap\n")
		}
	} else {
		fmt.Printf("    ⚠️  RunAI cluster version configmap not found\n")
	}

	// Check Helm releases (from Kubernetes secrets)
	secrets, err := c.clientset.CoreV1().Secrets("runai").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "owner=helm",
	})
	if err == nil && len(secrets.Items) > 0 {
		fmt.Printf("    ✅ %d Helm release(s) found in runai namespace\n", len(secrets.Items))
		for _, secret := range secrets.Items {
			if name := secret.Labels["name"]; name != "" {
				status := secret.Labels["status"]
				if status == "" {
					status = "unknown"
				}
				fmt.Printf("      - %s (status: %s)\n", name, status)
			}
		}
	} else if err != nil {
		fmt.Printf("    ⚠️  Could not check Helm releases: %v\n", err)
	} else {
		fmt.Printf("    ⚠️  No Helm releases found in runai namespace\n")
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

	output, err := c.getResourceAsYAML(namespace, canonicalType, workload)
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

	output, err := c.getResourceAsYAML(namespace, "rj", workload)
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

	pods, err := c.getPodsWithLabels(namespace, fmt.Sprintf("workloadName=%s", workload))
	if err != nil {
		return "", err
	}
	output, err := c.objectToYAML(pods)
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

	// PodGroups in RunAI have generated names, so we need to find them by labels
	podGroups, err := c.getPodGroupsWithLabels(namespace, fmt.Sprintf("workloadName=%s", workload))
	if err != nil {
		return "", fmt.Errorf("failed to search for PodGroups: %w", err)
	}

	if len(podGroups.Items) == 0 {
		return "", fmt.Errorf("PodGroup not found (this is normal for some workload types)")
	}

	// Convert the first PodGroup to YAML
	output, err := c.objectToYAML(podGroups)
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
	podList, err := c.getPodsWithLabels(namespace, fmt.Sprintf("workloadName=%s", workload))
	if err != nil {
		return nil, err
	}

	var pods []string
	for _, pod := range podList.Items {
		pods = append(pods, pod.Name)
	}
	if len(pods) == 0 {
		fmt.Printf("    ⚠️  No pods found for workload: %s\n", workload)
		return []string{}, nil
	}

	var outputFiles []string

	// Iterate through each pod
	for _, pod := range pods {
		fmt.Printf("    🐳 Processing pod: %s\n", pod)

		// Get all containers for this pod
		containers, initContainers, err := c.getPodContainers(namespace, pod)
		if err != nil {
			continue
		}

		// Combine init and regular containers
		allContainers := append(initContainers, containers...)

		// Iterate through each container
		for _, container := range allContainers {
			logFile := fmt.Sprintf("%s_%s_pod_logs_%s.log", workload, typeSafe, container)
			fmt.Printf("      📝 Getting logs for container: %s\n", container)

			output, err := c.getPodLogsForContainer(namespace, pod, container)
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

	output, err := c.getResourceAsYAML(namespace, "ksvc", workload)
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

// dumpSchedulerResource dumps a scheduler resource type using native client-go
func (c *Collector) dumpSchedulerResource(resourceType, singular string) error {
	fmt.Printf("📊 Dumping %s...\n", resourceType)

	// Map resource types to their actual GVR based on RunAI API definitions
	gvrCandidates := map[string][]schema.GroupVersionResource{
		"projects":    {{Group: "run.ai", Version: "v2", Resource: "projects"}},
		"queues":      {{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}},
		"nodepools":   {{Group: "run.ai", Version: "v1alpha1", Resource: "nodepools"}},
		"departments": {{Group: "scheduling.run.ai", Version: "v1", Resource: "departments"}},
	}

	gvrList, exists := gvrCandidates[resourceType]
	if !exists {
		return fmt.Errorf("unknown scheduler resource type: %s", resourceType)
	}

	// Get resource list using dynamic client with fallback versions
	listFile := fmt.Sprintf("%s_list.txt", resourceType)
	var resourceList *unstructured.UnstructuredList
	var lastErr error

	// Try each GVR version until one works
	for _, gvr := range gvrList {
		var err error
		resourceList, err = c.dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			break // Success
		}
		lastErr = err
	}

	if resourceList == nil {
		// If we can't list resources, create an informative error file instead of empty
		errorOutput := fmt.Sprintf("# %s resources\n# Error retrieving %s: %v\n# This may be normal if %s are not configured in this cluster\n",
			resourceType, resourceType, lastErr, resourceType)
		if err := os.WriteFile(listFile, []byte(errorOutput), 0644); err != nil {
			return fmt.Errorf("failed to write %s error file: %w", resourceType, err)
		}
		fmt.Printf("⚠️  %s list saved with error info to %s\n", resourceType, listFile)
		return nil
	}

	// Create list output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("# %s resources (found %d)\n", resourceType, len(resourceList.Items)))
	output.WriteString(fmt.Sprintf("# Retrieved using native Kubernetes client-go\n\n"))

	// Special handling for queues
	if resourceType == "queues" {
		output.WriteString("# Note: Queues are dedicated RunAI scheduling resources\n")
		output.WriteString("# API: scheduling.run.ai/v2\n\n")
	}

	output.WriteString("NAME\tCREATED\tAGE\n")

	resourceNames := []string{}
	for _, item := range resourceList.Items {
		name := item.GetName()

		// For queues, we may want to show additional scheduling info
		if resourceType == "queues" {
			// Queues are now proper scheduling.run.ai/v2 resources
			// Could add queue-specific metadata here if needed
		}

		resourceNames = append(resourceNames, name)

		creationTime := item.GetCreationTimestamp()
		age := time.Since(creationTime.Time).Truncate(time.Second)

		output.WriteString(fmt.Sprintf("%s\t%s\t%s\n",
			name,
			creationTime.Format("2006-01-02 15:04:05"),
			age.String()))
	}

	if err := os.WriteFile(listFile, []byte(output.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s list: %w", resourceType, err)
	}

	fmt.Printf("✅ %s list saved to %s (%d resources found)\n", resourceType, listFile, len(resourceList.Items))

	// Extract individual manifests
	if len(resourceNames) > 0 {
		fmt.Printf("📄 Extracting individual %s manifests...\n", resourceType)

		for _, resourceName := range resourceNames {
			manifestFile := fmt.Sprintf("%s_%s.yaml", singular, resourceName)

			// Get individual resource with fallback versions
			var resource *unstructured.Unstructured
			var resourceErr error

			for _, gvr := range gvrList {
				resource, resourceErr = c.dynamicClient.Resource(gvr).Get(context.TODO(), resourceName, metav1.GetOptions{})
				if resourceErr == nil {
					break // Success
				}
			}

			if resource == nil {
				fmt.Printf("  ⚠️  Failed to get %s %s: %v\n", singular, resourceName, resourceErr)
				continue
			}

			// Convert to YAML
			manifestOutput, err := c.objectToYAML(resource)
			if err != nil {
				fmt.Printf("  ⚠️  Failed to convert %s %s to YAML: %v\n", singular, resourceName, err)
				continue
			}

			if err := os.WriteFile(manifestFile, []byte(manifestOutput), 0644); err != nil {
				fmt.Printf("  ⚠️  Failed to write %s %s: %v\n", singular, resourceName, err)
				continue
			}

			fmt.Printf("  ✅ Extracted %s: %s\n", singular, resourceName)
		}
	} else {
		fmt.Printf("📄 No %s found to extract\n", resourceType)
	}

	return nil
}

// validateFileContent checks if a file has meaningful content (not just comments or empty)
func (c *Collector) validateFileContent(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", filename, err)
	}

	content := strings.TrimSpace(string(data))
	lines := strings.Split(content, "\n")

	meaningfulLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comment-only lines
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			meaningfulLines++
		}
	}

	if meaningfulLines == 0 {
		return fmt.Errorf("file %s contains no meaningful content (only comments or empty)", filename)
	}

	return nil
}
