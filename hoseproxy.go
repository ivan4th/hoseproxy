package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/tools/clientcmd"
)

var (
	master     = flag.String("master", "http://127.0.0.1:8080/", "apiserver URL")
	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	prefix     = flag.String("prefix", "hose-proxy", "service name prefix")
	src        = flag.String("src", "", "name of the default service")
	nServices  = flag.Int("nservices", 10, "max number of services to create in each goroutine")
	nSteps     = flag.Int("nsteps", 20, "maximum number of steps to take in each goroutine")
	nParallel  = flag.Int("nparallel", 1, "number of goroutines to launch")
)

func createService(clientset *kubernetes.Clientset, name string, srcEp *v1.Endpoints) {
	svc := &v1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol: "TCP",
					Port:     80,
				},
			},
		},
	}
	if _, err := clientset.Core().Services("default").Create(svc); err != nil {
		log.Printf("WARNING: Creating service: %v", err)
		return
	}

	endpoints := &v1.Endpoints{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
		Subsets: srcEp.Subsets,
	}
	if _, err := clientset.Core().Endpoints("default").Create(endpoints); err != nil {
		log.Printf("WARNING: Creating endpoints: %v", err)
	}
}

func deleteService(clientset *kubernetes.Clientset, name string, ignoreError bool) {
	// endpoints are deleted automagically
	// if err := clientset.Core().Endpoints("default").Delete(name, &api.DeleteOptions{}); !ignoreError && err != nil {
	// 	log.Printf("warning: deleting endpoints %q: %v", name, err)
	// }
	if err := clientset.Core().Services("default").Delete(name, &api.DeleteOptions{}); !ignoreError && err != nil {
		log.Printf("warning: deleting service %q: %v", name, err)
	}
}

func generateIds(base string) chan string {
	out := make(chan string)
	go func() {
		for i := 0; ; i++ {
			name := fmt.Sprintf("%s-%d", base, i)
			log.Printf("New: %q", name)
			out <- name
		}
	}()
	return out
}

func frobServices(clientset *kubernetes.Clientset, nServices, nSteps int, idCh chan string, stopCh chan struct{}, srcEp *v1.Endpoints) {
	names := make([]string, nServices)
	p := 0
loop:
	for i := 0; nSteps <= 0 || i < nSteps; i++ {
		select {
		case <-stopCh:
			break loop
		default:
		}
		if names[p] != "" {
			deleteService(clientset, names[p], false)
		}
		names[p] = <-idCh
		deleteService(clientset, names[p], true)
		createService(clientset, names[p], srcEp)
		p = (p + 1) % nServices
	}

	for _, name := range names {
		if name != "" {
			log.Printf("Cleanup: %q", name)
			deleteService(clientset, name, true)
		}
	}
}

func main() {
	flag.Parse()

	if *src == "" {
		log.Fatalf("specify the source service via -src")
	}

	config, err := clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	if err != nil {
		log.Fatalf("BuildConfigFromFlags(): %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("NewForConfig(): %v", err)
	}

	srcEp, err := clientset.Core().Endpoints("default").Get(*src)
	if err != nil {
		log.Fatalf("Getting endpoints: %v", err)
	}

	stopCh := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		close(stopCh)
	}()

	idCh := generateIds(*prefix)
	var wg sync.WaitGroup
	for i := 0; i < *nParallel; i++ {
		wg.Add(1)
		go func() {
			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				log.Fatalf("NewForConfig(): %v", err)
			}
			frobServices(clientset, *nServices, *nSteps, idCh, stopCh, srcEp)
			wg.Done()
		}()
	}
	wg.Wait()
}
