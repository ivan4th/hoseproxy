# K8s proxy load testing tool

This program was written as part of an attempt to reproduce
[a bug](https://github.com/kubernetes/kubernetes/issues/38372) that
makes kube-proxy hang.

Steps to install and use.

1. Download `hoseproxy` binary
```
wget https://github.com/ivan4th/hoseproxy/releases/download/0.1/hoseproxy
chmod +x hoseproxy
```
2. Pick an existing service (non-headless one pointing to a pod), let's say it's `nginx`.
3. Start `hoseproxy`:
```
./hoseproxy -master http://localhost:8080 -src nginx -nparallel 100 -nservices 100 -nsteps 10000
```
This particular command connects to apiserver on `http://localhost:8080`, uses `nginx` service
as the base for services and endpoints its creates. It uses 100 parallel goroutines
each of which creates 10000 services+endpoints in a loop, keeping at most 100 services
per goroutine at each moment (excess services are deleted).

Below is the list of important options to consider:
```
  -kubeconfig string
        absolute path to the kubeconfig file
  -master string
        apiserver URL (default "http://127.0.0.1:8080/")
  -nparallel int
        number of goroutines to launch (default 1)
  -nservices int
        max number of services to create in each goroutine (default 10)
  -nsteps int
        maximum number of steps to take in each goroutine (default 20)
  -prefix string
        service name prefix (default "hose-proxy")
  -src string
        name of the default service
```
