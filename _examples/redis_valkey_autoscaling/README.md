# Autoscaler for Redis/Valkey based worker application

This example shows how to deploy and test autoscaler with Redis or Valkey queue-based workers on a local Kubernetes cluster using k3d.

**Note**: This example uses simple Ubuntu images as workers - they won't actually process the queue. You'll manually add/remove jobs from the queue to simulate workload and observe autoscaling behavior.

- [Autoscaler for Redis/Valkey based worker application](#autoscaler-for-redisvalkey-based-worker-application)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
    - [1. Install k3d](#1-install-k3d)
    - [2. Create k3d Cluster](#2-create-k3d-cluster)
    - [3. Create Namespace](#3-create-namespace)
  - [Testing Redis Autoscaling](#testing-redis-autoscaling)
    - [1. Deploy Redis](#1-deploy-redis)
    - [2. Deploy Worker Pods](#2-deploy-worker-pods)
    - [3. Deploy Autoscaler](#3-deploy-autoscaler)
    - [4. Test Autoscaling](#4-test-autoscaling)
  - [Testing Valkey Autoscaling](#testing-valkey-autoscaling)
    - [1. Deploy Valkey (instead of Redis)](#1-deploy-valkey-instead-of-redis)
    - [2. Deploy Valkey Worker](#2-deploy-valkey-worker)
    - [3. Deploy Autoscaler with Valkey Config](#3-deploy-autoscaler-with-valkey-config)
    - [4. Test Valkey Autoscaling](#4-test-valkey-autoscaling)
  - [Cleanup](#cleanup)

## Prerequisites

1. **k3d** - Install via: `brew install k3d` (or appropriate package manager)
2. **kubectl** - Install via: `brew install kubectl` (usually comes with k3d)
3. **Redis CLI** (optional) - For local testing: `brew install redis`

**Note**: The autoscaler image is pulled from `ghcr.io/airhelp-osp/autoscaler:latest`, so no local Docker build is needed.

## Quick Start

### 1. Install k3d

```bash
# macOS
brew install k3d

# Linux
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Verify installation
k3d version
```

### 2. Create k3d Cluster

```bash
# Create cluster with 1 server and 2 agents
k3d cluster create autoscaler-test --servers 1 --agents 2 --wait

# Set default context
kubectl cluster-info
```

### 3. Create Namespace

```bash
# Create testing namespace
kubectl create namespace autoscaler-test
kubectl config set-context --current --namespace=autoscaler-test
```

---

## Testing Redis Autoscaling

### 1. Deploy Redis

```bash
# Create Redis deployment and service
kubectl apply -f - << 'REDIS'
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
REDIS

# Wait for Redis to be ready
kubectl rollout status deployment/redis --timeout=30s
```

### 2. Deploy Worker Pods

```bash
# Create dummy worker deployment (won't actually process jobs)
kubectl apply -f - << 'WORKER'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-worker
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis-worker
  template:
    metadata:
      labels:
        app: redis-worker
    spec:
      containers:
      - name: worker
        image: ubuntu:22.04
        command: ["/bin/sh", "-c"]
        args:
          - "echo 'Worker pod running'; sleep infinity"
        resources:
          requests:
            memory: "32Mi"
            cpu: "50m"
          limits:
            memory: "64Mi"
            cpu: "100m"
WORKER

# Wait for worker to be ready
kubectl rollout status deployment/redis-worker --timeout=30s
```

### 3. Deploy Autoscaler

```bash
# Create RBAC, ConfigMap, and Autoscaler deployment
kubectl apply -f - << 'AUTOSCALER'
apiVersion: v1
kind: ServiceAccount
metadata:
  name: autoscaler

---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: autoscaler-role
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "update"]
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["autoscaler-config"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["list"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: autoscaler-role-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: autoscaler-role
subjects:
  - kind: ServiceAccount
    name: autoscaler

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: autoscaler-config
data:
  redis-worker: |
    minimum_number_of_pods: 1
    maximum_number_of_pods: 5
    check_interval: 5s
    cooldown_period: 10s
    threshold: 10
    redis:
      hosts:
        - redis:6379
      list_keys:
        - job-queue

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autoscaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: autoscaler
  template:
    metadata:
      labels:
        app: autoscaler
    spec:
      serviceAccountName: autoscaler
      containers:
      - name: autoscaler
        image: ghcr.io/airhelp-osp/autoscaler:latest
        imagePullPolicy: Always
        args:
          - "--namespace=autoscaler-test"
          - "--environment=test"
          - "-v"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
AUTOSCALER

# Wait for autoscaler to be ready
kubectl rollout status deployment/autoscaler --timeout=30s
```

### 4. Test Autoscaling

**Terminal 1 - Monitor Status:**
```bash
# Watch replica count and queue size
watch -n 1 'echo "=== Workers ==="; kubectl get deployment redis-worker -o wide; echo ""; echo "=== Queue ==="; kubectl exec $(kubectl get pod -l app=redis -o jsonpath="{.items[0].metadata.name}") -- redis-cli LLEN job-queue'
```

**Terminal 2 - Add/Remove Jobs:**
```bash
# Get Redis pod name
REDIS_POD=$(kubectl get pod -l app=redis -o jsonpath='{.items[0].metadata.name}')

# Add 50 jobs to queue (one at a time)
for i in {1..50}; do
  kubectl exec $REDIS_POD -- redis-cli RPUSH job-queue "job-$i" > /dev/null
  echo "Added job $i"
  sleep 1
done

# Watch scaling happen in Terminal 1
# You should see workers scale from 1 → 5 replicas

# Clear the queue
kubectl exec $REDIS_POD -- redis-cli DEL job-queue
echo "Queue cleared"

# Watch workers scale back down to 1
```

---

## Testing Valkey Autoscaling

Follow the exact same steps as Redis above, but replace:

### 1. Deploy Valkey (instead of Redis)

```bash
kubectl apply -f - << 'VALKEY'
apiVersion: v1
kind: Service
metadata:
  name: valkey
spec:
  selector:
    app: valkey
  ports:
  - port: 6379
    targetPort: 6379

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: valkey
spec:
  replicas: 1
  selector:
    matchLabels:
      app: valkey
  template:
    metadata:
      labels:
        app: valkey
    spec:
      containers:
      - name: valkey
        image: valkey/valkey:latest
        ports:
        - containerPort: 6379
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
VALKEY

kubectl rollout status deployment/valkey --timeout=30s
```

### 2. Deploy Valkey Worker

```bash
kubectl apply -f - << 'WORKER'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: valkey-worker
spec:
  replicas: 1
  selector:
    matchLabels:
      app: valkey-worker
  template:
    metadata:
      labels:
        app: valkey-worker
    spec:
      containers:
      - name: worker
        image: ubuntu:22.04
        command: ["/bin/sh", "-c"]
        args:
          - "echo 'Worker pod running'; sleep infinity"
        resources:
          requests:
            memory: "32Mi"
            cpu: "50m"
          limits:
            memory: "64Mi"
            cpu: "100m"
WORKER

kubectl rollout status deployment/valkey-worker --timeout=30s
```

### 3. Deploy Autoscaler with Valkey Config

```bash
kubectl apply -f - << 'AUTOSCALER'
apiVersion: v1
kind: ServiceAccount
metadata:
  name: autoscaler

---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: autoscaler-role
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "update"]
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["autoscaler-config"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["list"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: autoscaler-role-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: autoscaler-role
subjects:
  - kind: ServiceAccount
    name: autoscaler

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: autoscaler-config
data:
  valkey-worker: |
    minimum_number_of_pods: 1
    maximum_number_of_pods: 5
    check_interval: 5s
    cooldown_period: 10s
    threshold: 10
    valkey:
      hosts:
        - valkey:6379
      list_keys:
        - job-queue

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autoscaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: autoscaler
  template:
    metadata:
      labels:
        app: autoscaler
    spec:
      serviceAccountName: autoscaler
      containers:
      - name: autoscaler
        image: ghcr.io/airhelp-osp/autoscaler:latest
        imagePullPolicy: Always
        args:
          - "--namespace=autoscaler-test"
          - "--environment=test"
          - "-v"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
AUTOSCALER

kubectl rollout status deployment/autoscaler --timeout=30s
```

### 4. Test Valkey Autoscaling

**Terminal 1 - Monitor:**
```bash
VALKEY_POD=$(kubectl get pod -l app=valkey -o jsonpath='{.items[0].metadata.name}')
watch -n 1 'echo "=== Workers ==="; kubectl get deployment valkey-worker -o wide; echo ""; echo "=== Queue ==="; kubectl exec $VALKEY_POD -- valkey-cli LLEN job-queue'
```

**Terminal 2 - Add/Remove Jobs:**
```bash
VALKEY_POD=$(kubectl get pod -l app=valkey -o jsonpath='{.items[0].metadata.name}')

# Add 50 jobs
for i in {1..50}; do
  kubectl exec $VALKEY_POD -- valkey-cli RPUSH job-queue "job-$i" > /dev/null
  echo "Added job $i"
  sleep 1
done

# Clear queue
kubectl exec $VALKEY_POD -- valkey-cli DEL job-queue
```

## Cleanup

```bash
# Delete cluster
k3d cluster delete autoscaler-test

# Verify cluster is deleted
k3d cluster list
```
