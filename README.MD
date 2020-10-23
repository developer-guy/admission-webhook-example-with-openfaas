# Prerequisites
* A Kubernetes cluster (kind, minikube, etc.)
* OpenFaaS CLI
* Arkade
* Kubectl
* KinD

## 2. Setup Tools
* Arkade
```sh
$ curl -sLS https://dl.get-arkade.dev | sudo sh
```

* KinD
```sh
$ arkade get kind
```

* Kubectl
```sh
$ arkade get kubectl
```

* OpenFaaS CLI
```sh
$ arkade get faas-cli
```

# Setup

## 1. Set Up a Kubernetes Cluster with Kind (Optional)

With Kind, you can run a local Kubernetes cluster using Docker containers as nodes. The steps in this section are optional. Follow them only if you don't have a running Kubernetes cluster.

Create a file named openfaas-cluster.yaml, and copy in the following spec:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
```

```bash
$ kind create cluster --config kind-specs/kind-cluster.yaml
```

* Deploy OpenFaaS to a Kubernetes Cluster with:

```sh
$ arkade install openfaas
```

* Verify that the deployments were created

```sh
$ kubectl get deployments -n openfaas -l "release=openfaas, app=openfaas"
```

## 3. Deploy Validating Admission Webhook

```sh
$ cd deployment
$ sh webhook-create-signed-cert.sh
$ export CA_BUNDLE=$(kubectl config view --minify --flatten -o json | jq -r '.clusters[] | select(.name == "'$(kubectl config current-context)'") | .cluster."certificate-authority-data"')
$ sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" validatingwebhook.yaml | kubectl apply -f -
$ cd ..
$ DOCKER_USER=username ./build
$ cd deployment
$ kubectl apply -f rbac.yaml
$ kubectl apply -f service.yaml
$ kubectl apply -f deployment.yaml # make sure you are using same 'DOCKER_USER' in deployment.yaml. i.e: devopps
```

## 4. Building OpenFaaS Function

```sh
$ cd functions
$ faas-cli up -f requiredlabel.yml # (build-push-deploy) make sure you are using your docker hub username. i.e: devopps
```

* Verify the functions that are working in `openfaas-fn` namespace.

## 5. Testing the whole workflow

* K8S API -> WebHook Broker w/TLS -> OpenFaaS Gateway (w/HTTP)

* The purpose of this PoC is that to validate that pods has required `labels`. Which means you must have that labels:

```yaml
app.kubernetes.io/name: sleep
app.kubernetes.io/instance: sleep
app.kubernetes.io/version: "0.1"
app.kubernetes.io/component: dummy
app.kubernetes.io/part-of: admission-webhook-example
app.kubernetes.io/managed-by: kubernetes
```

* Any Pod who have above labels is valid for us.
```sh
`./deployment/sleep.yaml` -> Incorrect, not-valid (We should deny this creation request.)
`./deployment/sleep-no-validation.yaml` -> Skip-validation (Based on `admission-webhook-example.qikqiak.com/validate: "false"` annotation, we skipped validation.)
`./deployment/sleep-with-labels.yaml` -> Correct, valid (We should accept this creation request.)
```

## 6. References
* https://appfleet.com/blog/create-serverless-functions-with-openfaas/
* https://github.com/morvencao/kube-mutating-webhook-tutorial
