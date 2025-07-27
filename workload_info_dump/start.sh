#!/bin/bash

set -e

# Check if kubectl exists and is accessible
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    echo "Please install kubectl and ensure it's accessible"
    exit 1
fi
echo "✅ kubectl found and accessible"

# Test kubectl connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: kubectl cannot connect to cluster"
    echo "Please check your kubeconfig and cluster connectivity"
    exit 1
fi
echo "✅ kubectl can connect to cluster"

# Function to map type aliases to canonical k8s resource names
get_canonical_type() {
  case "$1" in
    dinfw|distributedinferenceworkloads)
      echo "distributedinferenceworkloads" ;;
    dw|distributedworkloads)
      echo "distributedworkloads" ;;
    ew|externalworkloads)
      echo "externalworkloads" ;;
    infw|inferenceworkloads)
      echo "inferenceworkloads" ;;
    iw|interactiveworkloads)
      echo "interactiveworkloads" ;;
    tw|trainingworkloads)
      echo "trainingworkloads" ;;
    *)
      echo "" ;;
  esac
}

# Function to get workload YAML
get_workload_yaml() {
  local workload="$1"
  local canonical_type="$2"
  local type_safe="$3"
  
  local workload_yaml="${workload}_${type_safe}_workload.yaml"
  echo "  📄 Getting $canonical_type YAML..." >&2
  kubectl -n "$NAMESPACE" get "$canonical_type" "$workload" -o yaml > "$workload_yaml"
  if [[ $? -eq 0 ]]; then
    echo "    ✅ Workload YAML retrieved" >&2
    echo "$workload_yaml"
  else
    echo "    ❌ Failed to retrieve Workload YAML" >&2; exit 1
  fi
}

# Function to get RunAIJob YAML
get_runaijob_yaml() {
  local workload="$1"
  local type_safe="$2"
  
  local runaijob_yaml="${workload}_${type_safe}_runaijob.yaml"
  echo "  📄 Getting RunAIJob YAML..." >&2
  kubectl -n "$NAMESPACE" get rj "$workload" -o yaml > "$runaijob_yaml"
  if [[ $? -eq 0 ]]; then
    echo "    ✅ RunAIJob YAML retrieved" >&2
    echo "$runaijob_yaml"
  else
    echo "    ❌ Failed to retrieve RunAIJob YAML" >&2; exit 1
  fi
}

# Function to get pod YAML
get_pod_yaml() {
  local workload="$1"
  local type_safe="$2"
  
  local pod_yaml="${workload}_${type_safe}_pod.yaml"
  echo "  📄 Getting Pod YAML..." >&2
  kubectl -n "$NAMESPACE" get pod -l workloadName=$workload -o yaml > "$pod_yaml"
  if [[ $? -eq 0 ]]; then
    echo "    ✅ Pod YAML retrieved" >&2
    echo "$pod_yaml"
  else
    echo "    ❌ Failed to retrieve Pod YAML" >&2; exit 1
  fi
}

# Function to get podgroup YAML
get_podgroup_yaml() {
  local workload="$1"
  local type_safe="$2"
  
  local podgroup_yaml="${workload}_${type_safe}_podgroup.yaml"
  echo "  📄 Getting PodGroup YAML..." >&2
  kubectl -n "$NAMESPACE" get pg -l workloadName=$workload -o yaml > "$podgroup_yaml"
  if [[ $? -eq 0 ]]; then
    echo "    ✅ PodGroup YAML retrieved" >&2
    echo "$podgroup_yaml"
  else
    echo "    ❌ Failed to retrieve PodGroup YAML" >&2; exit 1
  fi
}

# Function to get pod logs
get_pod_logs() {
  local workload="$1"
  local type_safe="$2"
  
  echo "  📄 Getting Pod Logs..." >&2
  
  # Get all pods for this workload
  local pods=$(kubectl -n "$NAMESPACE" get pod -l workloadName=$workload -o jsonpath='{.items[*].metadata.name}')
  
  if [[ -z "$pods" ]]; then
    echo "    ⚠️  No pods found for workload: $workload" >&2
    return 0
  fi
  
  local output_files=()
  
  # Iterate through each pod
  for pod in $pods; do
    echo "    🐳 Processing pod: $pod" >&2
    
    # Get all containers (including init containers) for this pod
    local containers=$(kubectl -n "$NAMESPACE" get pod "$pod" -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}')
    
    # Iterate through each container
    for container in $containers; do
      local log_file="${workload}_${type_safe}_pod_logs_${container}.log"
      echo "      📝 Getting logs for container: $container" >&2
      
      kubectl -n "$NAMESPACE" logs "$pod" -c "$container" > "$log_file" 2>/dev/null
      if [[ $? -eq 0 ]]; then
        echo "        ✅ Container logs retrieved: $container" >&2
        output_files+=("$log_file")
      else
        echo "        ❌ Failed to retrieve logs for container: $container" >&2
      fi
    done
  done
  
  if [[ ${#output_files[@]} -gt 0 ]]; then
    echo "    ✅ Pod logs retrieved for ${#output_files[@]} containers" >&2
    printf '%s\n' "${output_files[@]}"
  else
    echo "    ❌ No container logs were successfully retrieved" >&2
    exit 1
  fi
}

# Function to get KSVC YAML (for inference workloads)
get_ksvc_yaml() {
  local workload="$1"
  local type_safe="$2"
  
  local ksvc_spec="${workload}_${type_safe}_ksvc.yaml"
  echo "  📄 Getting KSVC YAML..." >&2
  kubectl -n "$NAMESPACE" get ksvc "$workload" -o yaml > "$ksvc_spec"
  if [[ $? -eq 0 ]]; then
    echo "    ✅ KSVC YAML retrieved" >&2
    echo "$ksvc_spec"
  else
    echo "    ❌ Failed to retrieve KSVC YAML" >&2; exit 1
  fi
}

usage() {
  echo "Usage: $0 --project <PROJECT> --workload <WORKLOAD> --type <TYPE>"
  echo "Example: $0 --project test --type tw --workload test-train"
  echo "Example: $0 --project test --type iw --workload test-interactive"
  echo "Example: $0 --project test --type infw --workload test-inference"
  echo "Example: $0 --project test --type dinfw --workload test-distributed-inference"
  echo "Example: $0 --project test --type dw --workload test-distributed-training"
  echo "Example: $0 --project test --type ew --workload test-external"
  exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --project)
      PROJECT="$2"
      shift; shift
      ;;
    --workload)
      WORKLOAD="$2"
      shift; shift
      ;;
    --type)
      TYPE="$2"
      shift; shift
      ;;
    *)
      echo "Unknown argument: $1"
      usage
      ;;
  esac
done

# Validate required arguments
if [[ -z "$PROJECT" || -z "$WORKLOAD" || -z "$TYPE" ]]; then
  usage
fi

# Lookup namespace from project
NAMESPACE=$(kubectl get ns -l runai/queue="$PROJECT" -o jsonpath='{.items[0].metadata.name}')
if [[ -z "$NAMESPACE" ]]; then
  echo "No namespace found for project: $PROJECT"
  exit 1
fi

echo "Collecting info dump for workload '$WORKLOAD' ($TYPE) in project $PROJECT"
echo ""

# Validate type
CANONICAL_TYPE=$(get_canonical_type "$TYPE")
if [[ -z "$CANONICAL_TYPE" ]]; then
  echo "Invalid type: $TYPE"
  usage
fi

# Output file names
TYPE_SAFE=$(echo "$TYPE" | tr '/' '_')
OUTPUT_FILES=()

# Add timestamp for archive
TIMESTAMP=$(date +"%Y_%m_%d-%H_%M")
ARCHIVE="${PROJECT}_${TYPE_SAFE}_${WORKLOAD}_${TIMESTAMP}.tar.gz"

# Execute all collection functions
echo "📁 Starting collection process..."
echo ""

# Get workload YAML
workload_yaml=$(get_workload_yaml "$WORKLOAD" "$CANONICAL_TYPE" "$TYPE_SAFE")
OUTPUT_FILES+=("$workload_yaml")

# Get runaijob YAML
runaijob_yaml=$(get_runaijob_yaml "$WORKLOAD" "$TYPE_SAFE")
OUTPUT_FILES+=("$runaijob_yaml")

# Get pod YAML
pod_yaml=$(get_pod_yaml "$WORKLOAD" "$TYPE_SAFE")
OUTPUT_FILES+=("$pod_yaml")

# Get podgroup YAML
podgroup_yaml=$(get_podgroup_yaml "$WORKLOAD" "$TYPE_SAFE")
OUTPUT_FILES+=("$podgroup_yaml")

# Get pod logs
pod_logs_output=$(get_pod_logs "$WORKLOAD" "$TYPE_SAFE")
# Add each log file to the output files array
while IFS= read -r log_file; do
  if [[ -n "$log_file" ]]; then
    OUTPUT_FILES+=("$log_file")
  fi
done <<< "$pod_logs_output"

# Get ksvc for inference workloads
if [[ "$CANONICAL_TYPE" == "inferenceworkloads" ]]; then
  ksvc_spec=$(get_ksvc_yaml "$WORKLOAD" "$TYPE_SAFE")
  OUTPUT_FILES+=("$ksvc_spec")
fi

# Archive the files
tar -czf "$ARCHIVE" "${OUTPUT_FILES[@]}"

if [[ $? -eq 0 ]]; then
  echo ""
  echo "📦 Archive created: $ARCHIVE"
  
  # Clean up individual files
  echo ""
  echo "🧹 Cleaning up individual files..."
  for file in "${OUTPUT_FILES[@]}"; do
    if [[ -f "$file" ]]; then
      rm "$file"
      echo "  🗑️  Deleted: $file"
    fi
  done
  echo ""
  echo "✅ Cleanup completed. Only archive remains: $ARCHIVE"
else
  echo ""
  echo "❌ Failed to create archive. Individual files preserved."
  exit 1
fi
