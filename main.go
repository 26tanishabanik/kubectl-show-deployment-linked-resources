package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tufin/asciitree"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pflag "github.com/spf13/pflag"
)

var wg sync.WaitGroup

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return ""
}

func getKubeConfig() *string {
	var kubeConfig *string
	if home := HomeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "")
	}
	return kubeConfig
}

func ClientSetup() (*kubernetes.Clientset, error) {
	var kubeConfig *string
	if pflag.Lookup("kubeconfig") == nil {
		kubeConfig = getKubeConfig()
		config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
		if err != nil {
			fmt.Printf("Error in new client config: %s\n", err)
			return nil, err
		}
		clientset := kubernetes.NewForConfigOrDie(config)
		return clientset, nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", pflag.Lookup("kubeconfig").Value.String())
	if err != nil {
		fmt.Printf("Error in new client config: %s\n", err)
		return nil, err
	}
	clientset := kubernetes.NewForConfigOrDie(config)
	return clientset, nil

}

func GetService(clientSet *kubernetes.Clientset, deploymentName, namespace string, ch1 chan string) {
	deployment, err := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("error in get deployment: ", err)
		return
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		fmt.Println("error in getting label selector: ", err)
		return
	}
	service, err := clientSet.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		fmt.Println("error in getting service name: ", err)
		return
	}
	// fmt.Println("service name: ", service.Items[0].Name)
	ch1 <- service.Items[0].Name
	defer wg.Done()
}

func GetReplicaSet(clientSet *kubernetes.Clientset, deploymentName, namespace string, ch2 chan string) {
	deployment, err := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("error in getting deployment: ", err)
		return
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		fmt.Println("error in getting label selector: ", err)
		return
	}
	replicaSet, err := clientSet.AppsV1().ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		fmt.Println("error in getting replica set name: ", err)
		return
	}
	// fmt.Println("replicaset name: ", replicaSet.Items[0].Name)
	ch2 <- replicaSet.Items[0].Name
	defer wg.Done()
}

func GetConfigMap(clientSet *kubernetes.Clientset, deploymentName, namespace string, ch chan string) {
	deployment, _ := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})

	for _, ct := range deployment.Spec.Template.Spec.Containers {
		if ct.Env != nil {
			for _, en := range ct.Env {
				if en.ValueFrom != nil {
					ch <- en.ValueFrom.ConfigMapKeyRef.Name
				}
			}
		}
		if ct.EnvFrom != nil {
			for _, enf := range ct.EnvFrom {
				if enf.ConfigMapRef != nil {
					ch <- enf.ConfigMapRef.Name
				}
			}
		}
	}
	if deployment.Spec.Template.Spec.Volumes != nil {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.ConfigMap != nil {
				ch <- vol.ConfigMap.Name
			}
		}
	}
	defer wg.Done()
	ch <- "/done"
}

func GetSecret(clientSet *kubernetes.Clientset, deploymentName, namespace string, ch chan string) {
	deployment, _ := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})

	for _, ct := range deployment.Spec.Template.Spec.Containers {
		if ct.Env != nil {
			for _, en := range ct.Env {
				if en.ValueFrom != nil {
					ch <- en.ValueFrom.SecretKeyRef.Name
				}
			}
		}
		if ct.EnvFrom != nil {
			for _, enf := range ct.EnvFrom {
				if enf.SecretRef != nil {
					ch <- enf.SecretRef.Name
				}
			}
		}
	}
	if deployment.Spec.Template.Spec.Volumes != nil {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.Secret != nil {
				ch <- vol.Secret.SecretName
			}
		}
	}
	ch <- "/done"
	defer wg.Done()
}

func GetVolume(clientSet *kubernetes.Clientset, deploymentName, namespace string, ch chan string) {
	deployment, _ := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})

	if deployment.Spec.Template.Spec.Volumes != nil {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.Secret == nil && vol.ConfigMap == nil {
				if vol.Name != "" {
					ch <- vol.Name
				}
			}
		}
	}
	ch <- "/done"
	defer wg.Done()
}

func main() {
	var configMapList, volumeList, secretList []string
	deploymentNamespace := flag.String("n", "default", "kubernetes namespace")
	deploymentName := flag.String("d", "", "kubernetes deployment name")
	help := flag.Bool("h", false, "usage ")
	flag.Parse()
	if *help {
		fmt.Println("Name: ")
		fmt.Println("  kubectl deploymentTree - shows all the resources directly related to the deployment")
		fmt.Println("The following options are available:")
		fmt.Println("  -d: provide the name of the deployment")
		fmt.Println("  -n: provide the namespace for the deployment")
		fmt.Println("Usage: ")
		fmt.Println("  kubectl depoymentTree [flags] [options]")
		return
	}
	clientSet, err := ClientSetup()
	if err != nil {
		fmt.Println("error in creating client set: ", err)
		return
	}
	ch1 := make(chan string)
	ch2 := make(chan string)
	ch3 := make(chan string)
	ch4 := make(chan string)
	ch5 := make(chan string)
	wg.Add(1)
	go GetService(clientSet, *deploymentName, *deploymentNamespace, ch1)
	wg.Add(1)
	go GetReplicaSet(clientSet, *deploymentName, *deploymentNamespace, ch2)
	wg.Add(1)
	go GetConfigMap(clientSet, *deploymentName, *deploymentNamespace, ch3)
	wg.Add(1)
	go GetSecret(clientSet, *deploymentName, *deploymentNamespace, ch4)
	wg.Add(1)
	go GetVolume(clientSet, *deploymentName, *deploymentNamespace, ch5)
	go func() {
		wg.Wait()
	}()

	service := <-ch1
	replicaSet := <-ch2
	
	for {
		data := <-ch3
		if data != "/done" {
			configMapList = append(configMapList, data)
		} else {
			break
		}
	}

	for {
		data := <-ch4
		if data != "/done" {
			secretList = append(secretList, data)
		} else {
			break
		}
	}

	for {
		data := <-ch5
		if data != "/done" {
			volumeList = append(volumeList, data)
		} else {
			break
		}
	}

	var configMap, secret, volume string
	

	tree := asciitree.Tree{}
	configMapList = append(configMapList, "haha")
	configMapList = append(configMapList, "haha1")
	tree.Add(fmt.Sprintf("Deployment: %s/Service: ",*deploymentName))
	tree.Add(fmt.Sprintf("Deployment: %s/Service: /%s", *deploymentName, service))
	tree.Add(fmt.Sprintf("Deployment: %s/Replica Set: ", *deploymentName))
	tree.Add(fmt.Sprintf("Deployment: %s/Replica Set: /%s", *deploymentName, replicaSet))
	configMap = "Deployment: " + *deploymentName
	secret = "Deployment: " + *deploymentName
	volume = "Deployment: " + *deploymentName
	for i, v := range configMapList {
		if i == 0 {
			configMap = configMap + "/Config Maps: "
		}
		var configMap = configMap + "/" + v
		tree.Add(configMap)
	}
	for i, v := range secretList {
		if i == 0 {
			secret = secret + "/Secrets: "
		}
		secret = secret + "/" + v
		tree.Add(secret)
	}
	for i, v := range volumeList {
		if i == 0 {
			volume = volume + "/Volumes: "
		}
		volume = volume + "/" + v
		tree.Add(volume)
	}
	tree.Fprint(os.Stdout, true, "")
}
