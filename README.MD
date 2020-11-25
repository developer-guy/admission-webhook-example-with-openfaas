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

```bash
$ kind create cluster
```

* Deploy OpenFaaS to a Kubernetes Cluster with:

```sh
$ arkade install openfaas
```

* Verify that the deployments were created

```sh
$ kubectl get deployments -n openfaas -l "release=openfaas, app=openfaas"
```

## 3. Deploy Mutating Admission Webhook

```sh
$ cd deployment
$ sh webhook-create-signed-cert.sh
$ export CA_BUNDLE=$(kubectl config view --minify --flatten -o json | jq -r '.clusters[] | select(.name == "'$(kubectl config current-context)'") | .cluster."certificate-authority-data"')
$ sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" mutatingwebhook.yaml | kubectl apply -f -
$ cd ..
$ DOCKER_USER=username ./build
$ cd deployment
$ kubectl apply -f rbac.yaml
$ kubectl apply -f service.yaml
$ kubectl apply -f deployment.yaml # make sure you are using same 'DOCKER_USER' in deployment.yaml. i.e: devopps
# Label the default namespace to enable the webhook
$ kubectl label namespaces default admission-webhook-example=enabled
```

## 4. Building OpenFaaS Function

```sh
$ cd functions
$ faas-cli up -f fileinjector.yml # (build-push-deploy) make sure you are using your docker hub username. i.e: devopps
```

* Verify the functions that are working in `openfaas-fn` namespace.

## 5. Testing the whole workflow

#### K8S API -> WebHook Broker w/TLS -> OpenFaaS Gateway (w/HTTP) --> OpenFaaS Function

* The purpose of this PoC is the webhook will set the default limit/request if the containers within the Deployment does not have resources.

* First, check the [deployment](deployment/busybox.yaml) manifest and see there is no resources defined for the container.

```sh
apiersion: apps/v1
kind: Deployment
metadata:
  name: busybox
  namespace: default
  labels:
    app: busybox
spec:
  selector:
    matchLabels:
      app: busybox
  replicas: 1
  template:
    metadata:
      labels:
        app: busybox
    spec:
      containers:
        - name: busybox
          image: busybox
          command: [ "/bin/sh" ]
          args: [ "-c", "while true; do cat /etc/config/hello-openfaas.txt; sleep 2; done" ]
```

* Then, you can apply the Deployment manifest.After doing this, you will notice that container resources are injected by the mutating webhook with the default values defined by the mutating webhook.

```sh
$ kubectl apply -f busybox.yaml
```

* Check the YAML of the Deployment, you will see the default resources.

```sh
$ kubectl get deployments busybox -ojsonpath='{.spec.template.spec.containers[?(@.name=="busybox")].resources}'
{"limits":{"cpu":"125Mi","memory":"75Mi"},"requests":{"cpu":"100Mi","memory":"50Mi"}}
```

## 6. References
* https://appfleet.com/blog/create-serverless-functions-with-openfaas/
* https://github.com/didil/k8s-hello-mutating-webhook
* https://medium.com/@didil/building-a-kubernetes-mutating-admission-webhook-7e48729523ed
