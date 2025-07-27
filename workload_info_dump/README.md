# Workload Info Dump

This script (`start.sh`) collects and archives information about a specific Run:AI workload and its related resources in a given Run:AI project (queue).

## Usage

### Directly from github (faster):
```sh
curl -s https://raw.githubusercontent.com/runai-professional-services/utilities/refs/heads/main/workload_info_dump/start.sh | bash -s -- --project <PROJECT> --type <TYPE> --workload <WORKLOAD>
```

Example:
```sh
curl -s https://raw.githubusercontent.com/runai-professional-services/utilities/refs/heads/main/workload_info_dump/start.sh | bash -s -- --project test --type tw --workload test-train
```

### Locally:
```sh
./start.sh --project <PROJECT> --type <TYPE> --workload <WORKLOAD>
```

Example:
```sh
./start.sh --project test --type tw --workload test-train
```

- `<PROJECT>`: The Run:AI project name. The script will automatically resolve the correct Kubernetes namespace.
- `<WORKLOAD>`: The name of the workload.
- `<TYPE>`: The type or alias of the workload resource. Allowed values:
  - Interactive: `iw`, `interactiveworkloads`
  - Training: `tw`, `trainingworkloads`
  - Distributed Training: `dw`, `distributedworkloads`
  - Inference: `infw`, `inferenceworkloads`
  - Distributed Inference: `dinfw`, `distributedinferenceworkloads`

## What it does
1. Resolves the Kubernetes namespace for the given project.
2. Dumps the following resources related to the workload as YAML or text files:
   - Workload YAML (`<WORKLOAD>_<TYPE>_workload.yaml`)
   - RunAIJob YAML (`<WORKLOAD>_<TYPE>_runaijob.yaml`)
   - Pod YAML (`<WORKLOAD>_<TYPE>_pod.yaml`)
   - PodGroup YAML (`<WORKLOAD>_<TYPE>_podgroup.yaml`)
   - Pod logs (`<WORKLOAD>_<TYPE>_pod_logs.txt`)
3. Archives all collected files into `<NAMESPACE>_<TYPE>_<WORKLOAD>_<TIMESTAMP>.tar.gz` (timestamp format: `yyyy_mm_dd-hh_mm`).
4. Prints clear, human-friendly status messages for each step, indicating success or failure.
5. Exits immediately if any step fails.
6. Prints a summary message at the start indicating what is being collected.

## Example

```sh
./start.sh --project test --workload test-workload --type iw
```

This will create an archive named like `runai-test_iw_test-workload_2024_06_07-15_30.tar.gz` containing:
- `test-workload_iw_workload.yaml`
- `test-workload_iw_runaijob.yaml`
- `test-workload_iw_pod.yaml`
- `test-workload_iw_podgroup.yaml`
- `test-workload_iw_pod_logs.txt`
