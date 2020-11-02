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

* K8S API -> WebHook Broker w/TLS -> OpenFaaS Gateway (w/HTTP) --> OpenFaaS Function

* The purpose of this PoC is that to add a simple volume + volumeMount to the pod:

* First, we need to create a configmap to be able to mount a file to the Pod:

```
$ kubectl create hello-configmap --from-file=functions/fileinjector/hello-openfaas.txt
```

* Now , you can apply the Pod manifest, once you done this , you will notice that Pod includes a volume + volumeMount additional to the manifest you wrote

```sh
$ kubectl apply -f busybox.yaml
```

* Check the logs of the Pod, you will see "Hello OpenFaaS" text.

```sh
$ kubectl logs -f busybox
```

## 6. References
* https://appfleet.com/blog/create-serverless-functions-with-openfaas/
* https://github.com/didil/k8s-hello-mutating-webhook
* https://medium.com/@didil/building-a-kubernetes-mutating-admission-webhook-7e48729523ed
