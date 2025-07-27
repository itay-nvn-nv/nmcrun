# Example output of 'nmcrun test' with a working RunAI cluster:

🧪 Running environment tests for RunAI log collection...

🔧 Testing required tools...
  🔍 Checking kubectl... ✅ Client Version: v1.28.2
  �� Checking helm... ✅ v3.12.0+gd77ceab

🌐 Testing cluster connectivity...
  🔗 Testing kubectl cluster connection... ✅ CONNECTED
  👥 Testing cluster permissions... ✅ SUFFICIENT
  📍 Current context: arn:aws:eks:us-west-2:123456789:cluster/runai-cluster
  🎯 Kubernetes control plane is running at https://A1B2C3D4E5.gr7.us-west-2.eks.amazonaws.com

📋 Checking RunAI namespaces...
  📂 Checking namespace 'runai'... ✅ EXISTS
    📦 12 pods found
  �� Checking namespace 'runai-backend'... ✅ EXISTS  
    📦 8 pods found
  ✅ Found 2 RunAI namespace(s): runai, runai-backend

📊 Retrieving RunAI cluster information...
  🔍 Extracting RunAI configuration...
  🌐 Cluster URL: https://company.run.ai
  🎛️  Control Plane URL: https://backend.company.run.ai
  📊 Checking RunAI components...
    ✅ RunAI configuration found
    📋 RunAI version: 2.15.23
    ✅ 2 Helm chart(s) found in runai namespace
      - runai-cluster (deployed)
      - prometheus-adapter (deployed)

🎉 All tests passed! Environment is ready for log collection.

Run 'nmcrun logs' to start collecting logs.
