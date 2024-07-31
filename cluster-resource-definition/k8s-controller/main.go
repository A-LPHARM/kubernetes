package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig string

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Allow the kubeconfig file to be specified via a flag
    flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "absolute path to the kubeconfig file")
    flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Println("Falling back to in-cluster config")
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Failed to get in-cluster config: %v", err)
		}
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create dynamic client: %v", err)
	}

	pdfdoc := schema.GroupVersionResource{Group: "alpharm.henry.com", Version: "v1", Resource: "pdfdocs"}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return dynClient.Resource(pdfdoc).Namespace("").List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return dynClient.Resource(pdfdoc).Namespace("").Watch(context.Background(), options)
			},
		},
		&unstructured.Unstructured{},
		0,
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Println("Add event detected:", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			log.Println("Update event detected:", newObj)
		},
		DeleteFunc: func(obj interface{}) {
			log.Println("Delete event detected:", obj)
		},
	})

	stop := make(chan struct{})
	defer close(stop)

	go informer.Run(stop)

	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		log.Fatalf("Timeout waiting for cache sync")
	}

	log.Println("Custom Resource Controller started successfully")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
