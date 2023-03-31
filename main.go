package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	
	pflag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// "github.com/m1gwings/treedrawer/tree"
	"github.com/tufin/asciitree"
)

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return ""
}

func getKubeConfig() *string{
	var kubeConfig *string
	if home := HomeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube","config"), "")
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
			return nil,err
		}
		clientset := kubernetes.NewForConfigOrDie(config)
		return clientset, nil
	}
	
	config, err := clientcmd.BuildConfigFromFlags("", pflag.Lookup("kubeconfig").Value.String())
	if err != nil {
		fmt.Printf("Error in new client config: %s\n", err)
		return nil,err
	}
	clientset := kubernetes.NewForConfigOrDie(config)
	return clientset, nil

}

func GetConfigMaps(clientSet *kubernetes.Clientset, deploymentName, namespace string) ([]string, error) {
	var configMapList []string
	deployment, _ := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	
	for _, ct := range deployment.Spec.Template.Spec.Containers {
		if ct.Env != nil {
			for _, en := range ct.Env {
				if en.ValueFrom != nil {
					// fmt.Println("Env Config Map: ", en.ValueFrom.ConfigMapKeyRef)
					configMapList = append(configMapList, en.ValueFrom.ConfigMapKeyRef.Name)
				}
			}
		}
		if ct.EnvFrom != nil {
			for _, enf := range ct.EnvFrom {
				if enf.ConfigMapRef != nil {
					// fmt.Println("EnvFrom Config Map: ", enf.ConfigMapRef.Name)
					configMapList = append(configMapList, enf.ConfigMapRef.Name)
				}
			}
		}
	}
	if deployment.Spec.Template.Spec.Volumes != nil {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.ConfigMap != nil {
				// fmt.Println("Volume Config Map: ", vol.ConfigMap.Name)
				configMapList = append(configMapList, vol.ConfigMap.Name)
			}
		}
	}
	return configMapList, nil
}

func GetSecrets(clientSet *kubernetes.Clientset, deploymentName, namespace string) ([]string, error) {
	var secretList []string
	deployment, _ := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	
	for _, ct := range deployment.Spec.Template.Spec.Containers {
		if ct.Env != nil {
			for _, en := range ct.Env {
				if en.ValueFrom != nil {
					// fmt.Println("Env Config Map: ", en.ValueFrom.ConfigMapKeyRef)
					secretList = append(secretList, en.ValueFrom.SecretKeyRef.Name)
				}
			}
		}
		if ct.EnvFrom != nil {
			for _, enf := range ct.EnvFrom {
				if enf.SecretRef != nil {
					// fmt.Println("EnvFrom Config Map: ", enf.ConfigMapRef.Name)
					secretList = append(secretList, enf.SecretRef.Name)
				}
			}
		}
	}
	if deployment.Spec.Template.Spec.Volumes != nil {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.Secret != nil {
				// fmt.Println("Volume secret: ", vol.Secret.SecretName)
				secretList= append(secretList, vol.Secret.SecretName)
			}
		}
	}
	return secretList, nil
}

func GetVolumes(clientSet *kubernetes.Clientset, deploymentName, namespace string) ([]string, error) {
	var volumeList []string
	deployment, _ := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), deploymentName, metav1.GetOptions{})
	
	
	if deployment.Spec.Template.Spec.Volumes != nil {
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.Secret == nil && vol.ConfigMap == nil  {
				// fmt.Println("Volume: ", vol.Name)
				if vol.Name != ""{
					volumeList= append(volumeList, vol.Name)
				}	
			}
		}
	}
	return volumeList, nil
}

func GetResourcesRelatedToDeployment(deploymentName, namespace string) (string, string, []string, []string, []string, error) {
	clientSet, err :=  ClientSetup()
	if err != nil {
		fmt.Println("error in creating client set: ", err)
		return "", "", nil, nil, nil, err
	}

	deployment, err := clientSet.AppsV1().Deployments("default").Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("error in get deployment: ", err)
		return  "", "", nil, nil, nil, err
	}
	
	selector, _ := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	// fmt.Println("selector: ", selector)
	service, err := clientSet.CoreV1().Services("default").List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	// fmt.Println("service name: ", service.Items[0].Name)
	if err != nil {
		fmt.Println("error in getting service name: ", err)
		return  "", "", nil, nil, nil, err
	}
	replicaSet, err := clientSet.AppsV1().ReplicaSets("default").List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	// fmt.Println("replica set name: ", replicaSet.Items[0].Name)
	if err != nil {
		fmt.Println("error in get replica sets: ", err)
		return  "", "", nil, nil, nil, err
	}
	configMapList, err := GetConfigMaps(clientSet, deployment.Name, namespace)
	if err != nil {
		fmt.Println("error in get config map list: ", err)
		return  "", "", nil, nil, nil, err
	}
	secretList, err := GetSecrets(clientSet, deployment.Name, namespace)
	if err != nil {
		fmt.Println("error in get secret list: ", err)
		return  "", "", nil, nil, nil, err
	}
	volumeList, err := GetVolumes(clientSet, deployment.Name, namespace)
	if err != nil {
		fmt.Println("error in get secret list: ", err)
		return  "", "", nil, nil, nil, err
	}
	return service.Items[0].Name, replicaSet.Items[0].Name, configMapList, secretList, volumeList, nil
}

func main() {
	var configMap, secret, volume string
	deploymentNamespace := flag.String("n", "default", "kubernetes namespace")
	deploymentName := flag.String("d", "", "kubernetes deployment name")
	flag.Parse()
	service, replicaSet, configMapList, secretList, volumeList, err := GetResourcesRelatedToDeployment(*deploymentName, *deploymentNamespace)
	if err != nil {
		fmt.Println("error in getting all the resources linked to a deployment: ", err)
		return
	}


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




