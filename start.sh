#!/bin/bash

# Function to check if a command exists
check_command() {
  local cmd=$1
  if ! command -v "$cmd" &> /dev/null; then
    echo "ERROR: '$cmd' command not found. Please install $cmd and try again."
    exit 1
  fi
}

# Function to collect logs for a given namespace and output directory
collect_logs() {
  local NAMESPACE=$1
  local LOG_DIR=$2
  local LOGS_SUBDIR="$LOG_DIR/logs"
  mkdir -p "$LOGS_SUBDIR"
  
  echo "  Collecting pod information for namespace: $NAMESPACE"
  PODS=$(kubectl get pods -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}')
  
  if [ -z "$PODS" ]; then
    echo "  No pods found in namespace: $NAMESPACE"
    return
  fi
  
  echo "  Found $(echo $PODS | wc -w) pods in namespace: $NAMESPACE"
  
  for POD in $PODS; do
    echo "  Processing pod: $POD"
    
    # Get regular containers
    CONTAINERS=$(kubectl get pod $POD -n $NAMESPACE -o jsonpath='{.spec.containers[*].name}')
    echo "    Regular containers found: $(echo $CONTAINERS | wc -w)"
    
    # Get init containers
    INIT_CONTAINERS=$(kubectl get pod $POD -n $NAMESPACE -o jsonpath='{.spec.initContainers[*].name}')
    echo "    Init containers found: $(echo $INIT_CONTAINERS | wc -w)"
    
    # Collect logs for regular containers
    for CONTAINER in $CONTAINERS; do
      LOG_FILE="$LOGS_SUBDIR/${POD}_${CONTAINER}.log"
      echo "    Collecting logs for Pod: $POD, Container: $CONTAINER"
      kubectl logs --timestamps $POD -c $CONTAINER -n $NAMESPACE > "$LOG_FILE" 2>/dev/null
      if [ $? -eq 0 ]; then
        echo "      ✓ Logs saved to: $LOG_FILE"
      else
        echo "      ⚠ Warning: Failed to collect logs for container: $CONTAINER"
      fi
    done
    
    # Collect logs for init containers
    for CONTAINER in $INIT_CONTAINERS; do
      LOG_FILE="$LOGS_SUBDIR/${POD}_${CONTAINER}_init.log"
      echo "    Collecting logs for Pod: $POD, Init Container: $CONTAINER"
      kubectl logs --timestamps $POD -c $CONTAINER -n $NAMESPACE > "$LOG_FILE" 2>/dev/null
      if [ $? -eq 0 ]; then
        echo "      ✓ Init container logs saved to: $LOG_FILE"
      else
        echo "      ⚠ Warning: Failed to collect logs for init container: $CONTAINER"
      fi
    done
  done
}

# Check for required tools
echo "Checking for required tools..."
check_command "kubectl"
check_command "helm"
echo "✓ All required tools are available"
echo ""

# Namespaces to check
NAMESPACES=("runai-backend" "runai")

echo "Extracting cluster information..."
CLUSTER_URL=$(kubectl -n runai get runaiconfig runai -o jsonpath='{.spec.__internal.global.clusterURL}' 2>/dev/null)
CP_URL=$(kubectl -n runai get runaiconfig runai -o jsonpath='{.spec.__internal.global.controlPlane.url}' 2>/dev/null)

if [ -z "$CLUSTER_URL" ]; then
  echo "⚠ Warning: Could not extract Cluster URL"
  CLUSTER_URL="unknown"
fi

if [ -z "$CP_URL" ]; then
  echo "⚠ Warning: Could not extract Control Plane URL"
  CP_URL="unknown"
fi

CP_NAME_CLEAN=$(echo "$CP_URL" | sed 's/https:\/\///; s/\./-/g')

echo "Cluster URL: $CLUSTER_URL"
echo "Control Plane URL: $CP_URL"
echo "Control Plane Name (cleaned): $CP_NAME_CLEAN"
echo "=========================================="

for NAMESPACE in "${NAMESPACES[@]}"; do
  echo ""
  echo "Processing namespace: $NAMESPACE"
  echo "----------------------------------------"
  
  # Check namespace existence before any operations
  echo "Checking if namespace '$NAMESPACE' exists..."
  kubectl get namespace "$NAMESPACE" >/dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "❌ Namespace '$NAMESPACE' does not exist. Skipping."
    continue
  fi

  echo "✓ Namespace '$NAMESPACE' exists. Starting log collection..."
  
  TIMESTAMP=$(date +%d-%m-%Y_%H-%M)
  LOG_NAME="$CP_NAME_CLEAN-$NAMESPACE-logs-$TIMESTAMP"
  LOG_DIR="./$LOG_NAME"
  mkdir $LOG_DIR
  SCRIPT_LOG="$LOG_DIR/script.log"
  LOG_ARCHIVE_NAME="$LOG_NAME.tar.gz"
  
  echo "Log directory: $LOG_DIR"
  echo "Log archive name: $LOG_ARCHIVE_NAME"
  echo "Script log: $SCRIPT_LOG"
  echo ""

  # Start logging all output for this namespace to script.log
  {
    echo "=== Log Collection Started at $(date) ==="
    echo "Namespace: $NAMESPACE"
    echo "Cluster URL: $CLUSTER_URL"
    echo "Control Plane URL: $CP_URL"
    echo ""
    
    # Collect logs into logs subdirectory
    echo "=== Collecting Pod Logs ==="
    collect_logs $NAMESPACE $LOG_DIR
    echo ""

    # Collect extra info per namespace
    echo "=== Collecting Additional Information ==="
    if [ "$NAMESPACE" == "runai" ]; then
      echo "Collecting Helm charts list..."
      helm ls -A > "$LOG_DIR/helm_charts_list.txt" 2>/dev/null
      echo "  ✓ Helm charts list saved"
      
      echo "Collecting Helm values for runai-cluster..."
      helm -n runai get values runai-cluster > "$LOG_DIR/helm-values_runai-cluster.yaml" 2>/dev/null
      echo "  ✓ Helm values saved"
      
      echo "Collecting ConfigMap runai-public..."
      kubectl -n runai get cm runai-public -o yaml > "$LOG_DIR/cm_runai-public.yaml" 2>/dev/null
      echo "  ✓ ConfigMap saved"
      
      echo "Collecting pod list for runai namespace..."
      kubectl -n runai get pods -o wide > "$LOG_DIR/pod-list_runai.txt" 2>/dev/null
      echo "  ✓ Pod list saved"
      
      echo "Collecting node list..."
      kubectl get nodes -o wide > "$LOG_DIR/node-list.txt" 2>/dev/null
      echo "  ✓ Node list saved"
      
      echo "Collecting RunAI config..."
      kubectl -n runai get runaiconfig runai -o yaml > "$LOG_DIR/runaiconfig.yaml" 2>/dev/null
      echo "  ✓ RunAI config saved"
      
      echo "Collecting engine config..."
      kubectl -n runai get configs.engine.run.ai engine-config -o yaml > "$LOG_DIR/engine-config.yaml" 2>/dev/null
      echo "  ✓ Engine config saved"
      
    elif [ "$NAMESPACE" == "runai-backend" ]; then
      echo "Collecting pod list for runai-backend namespace..."
      kubectl -n runai-backend get pods -o wide > "$LOG_DIR/pod-list_runai-backend.txt" 2>/dev/null
      echo "  ✓ Pod list saved"
      
      echo "Collecting Helm values for runai-backend..."
      helm -n runai-backend get values runai-backend > "$LOG_DIR/helm-values_runai-backend.yaml" 2>/dev/null
      echo "  ✓ Helm values saved"
    fi

    echo ""
    echo "=== Creating Archive ==="
    echo "Calculating directory size..."
    du -hs $LOG_DIR
    
    echo "Creating tar archive..."
    tar cvzf $LOG_ARCHIVE_NAME $LOG_DIR
    echo "  ✓ Archive created"
    
    echo "Archive details:"
    ls -lah $LOG_ARCHIVE_NAME
    
    echo "Cleaning up temporary directory..."
    rm -rf $LOG_DIR
    echo "  ✓ Temporary directory removed"
    
    echo "=== Log Collection Completed at $(date) ==="
    echo "Logs and info archived to $LOG_ARCHIVE_NAME"
    
  } 2>&1 | tee "$SCRIPT_LOG"
  
  echo ""
  echo "✓ Completed processing namespace: $NAMESPACE"
  echo "Archive created: $LOG_ARCHIVE_NAME"
  echo "=========================================="
done

echo ""
echo "🎉 All namespaces processed successfully!"