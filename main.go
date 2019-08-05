package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pmacik/k8s-rds/pkg/client"
	"github.com/pmacik/k8s-rds/pkg/crd"
	"github.com/pmacik/k8s-rds/pkg/kube"
	"github.com/pmacik/k8s-rds/pkg/provider"
	"github.com/pmacik/k8s-rds/pkg/rds"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const Failed = "Failed"

// return rest config, if path not specified assume in cluster config
func getClientConfig(kubeconfig string) (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		if kubeconfig != "" {
			return clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}
	return cfg, err
}

func getKubectl() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Println("Appears we are not running in a cluster")
		config, err = clientcmd.BuildConfigFromFlags("", kube.Config())
		if err != nil {
			return nil, err
		}
	} else {
		log.Println("Seems like we are running in a Kubernetes cluster!!")
	}

	kubectl, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return kubectl, nil
}

func main() {
	var provider string
	var rootCmd = &cobra.Command{
		Use:   "k8s-rds",
		Short: "Kubernetes database provisioner",
		Long:  `Kubernetes database provisioner`,
		Run: func(cmd *cobra.Command, args []string) {
			execute(provider)
		},
	}
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "aws", "Type of provider (aws)")
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}

func execute(dbprovider string) {
	log.Println("Starting k8s-rds")

	config, err := getClientConfig(kube.Config())
	if err != nil {
		panic(err.Error())
	}

	// create clientset and create our CRD, this only need to run once
	clientset, err := apiextcs.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// note: if the CRD exist our CreateCRD function is set to exit without an error
	_, err = crd.CreateCRD(clientset)
	if err != nil {
		panic(err)
	}

	// Create a new clientset which include our CRD schema
	restClient, scheme, err := crd.NewRESTClient(config)
	if err != nil {
		panic(err)
	}

	// Create a CRD client interface
	crClient := client.NewCRClient(restClient, scheme, "")
	log.Println("Watching for database changes...")
	_, controller := cache.NewInformer(
		crClient.NewListWatch(),
		&crd.Database{},
		time.Minute*2,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				db := obj.(*crd.Database)
				provider, err := getProvider(db, dbprovider)
				if err != nil {
					log.Printf("unable to get DB provider %s: %v", dbprovider, err)
					return
				}
				crClient := client.NewCRClient(restClient, scheme, db.Namespace) // add the database namespace to the client
				// create DB
				hostname, err := handleCreateDatabase(db, crClient, &provider)
				if err != nil {
					log.Printf("database creation failed: %v", err)
					err := updateStatus(db.Name, crd.DatabaseStatus{Message: fmt.Sprintf("%v", err), State: Failed}, crClient)
					if err != nil {
						log.Printf("database CRD status update failed: %v", err)
						return
					}
				}
				status := crd.DatabaseStatus{
					Message:            "DB Created - creating service",
					State:              "CreatingService",
					DBConnectionConfig: "",
					DBCredentials:      db.Spec.Password.Name,
				}
				err = updateStatus(db.Name, status, crClient)
				if err != nil {
					log.Printf("Unable to update status: %v", err)
					return
				}
				// create service
				log.Printf("Creating service '%v' for %v\n", db.Name, hostname)
				svc, err := provider.CreateService(db.Namespace, hostname, db.Name, db)
				if err != nil {
					if errors.IsAlreadyExists(err) {
						log.Printf("Service %s already exists, moving on", db.Name)
					} else {
						log.Printf("Unable to create service: %v", err)
						return
					}
				}
				status.Message = "Service Created"
				status.State = "CreatingConfigMap"
				err = updateStatus(db.Name, status, crClient)
				if err != nil {
					log.Printf("Unable to update status: %v", err)
					return
				}

				//create config map
				cfm, err := ensureConfigMap(db, svc)
				status.Message = "ConfigMap Created"
				status.State = "Completed"
				status.DBConnectionConfig = cfm.GetName()
				err = updateStatus(db.Name, status, crClient)
				if err != nil {
					log.Printf("Unable to update status: %v", err)
					return
				}

				log.Printf("Creation of database %v done\n", db.Name)
			},
			DeleteFunc: func(obj interface{}) {
				db := obj.(*crd.Database)
				log.Printf("deleting database: %s \n", db.Name)

				provider, err := getProvider(db, dbprovider)
				if err != nil {
					log.Printf("unable to get DB provider %s: %v", dbprovider, err)
					return
				}

				err = provider.DeleteDatabase(db)
				if err != nil {
					log.Println(err)
				}

				err = provider.DeleteService(db.Namespace, db.Name)
				if err != nil {
					log.Println(err)
				}
				log.Printf("Deletion of database %v done\n", db.Name)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}

func getProvider(db *crd.Database, dbprovider string) (provider.DatabaseProvider, error) {
	kubectl, err := getKubectl()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	switch dbprovider {
	case "aws":
		r, err := rds.New(db, kubectl)
		if err != nil {
			return nil, err
		}
		return r, nil

		/*case "local":
		r, err := local.New(db, kubectl)
		if err != nil {
			return nil, err
		}
		return r, nil*/
	}
	return nil, fmt.Errorf("unable to find provider for %v", dbprovider)
}

func handleCreateDatabase(db *crd.Database, crClient *client.CRClient, dbProvider *provider.DatabaseProvider) (string, error) {
	// validate dbname is only alpha numeric
	err := updateStatus(db.Name, crd.DatabaseStatus{Message: "Creating", State: "Creating"}, crClient)
	if err != nil {
		return "", fmt.Errorf("database CR status update failed: %v", err)
	}

	log.Println("Attempting to Create a DB")

	hostname, err := (*dbProvider).CreateDatabase(db)
	if err != nil {
		return "", err
	}
	return hostname, nil
}

func ensureConfigMap(db *crd.Database, svc *v1.Service) (*v1.ConfigMap, error) {
	//Create a configMap
	kubectl, err := kube.Client()
	if err != nil {
		return nil, err
	}
	configMapInterface := kubectl.CoreV1().ConfigMaps(db.Namespace)
	cfm, cErr := configMapInterface.Get(db.Name, metav1.GetOptions{})

	createCM := false
	if cErr != nil {
		cfm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      db.Name,
				Namespace: db.Namespace,
				Labels:    svc.ObjectMeta.Labels,
			},
			Data: map[string]string{
				"DB_HOST": svc.Spec.ExternalName,
				"DB_PORT": svc.Spec.Ports[0].TargetPort.String(),
			},
		}
		createCM = true
	}
	cfm.SetOwnerReferences(svc.GetOwnerReferences())
	if createCM {
		_, err = configMapInterface.Create(cfm)
	} else {
		_, err = configMapInterface.Update(cfm)
	}
	return cfm, nil
}

func updateStatus(dbName string, status crd.DatabaseStatus, crClient *client.CRClient) error {

	db, err := crClient.Get(dbName)
	if err != nil {
		return err
	}
	db.Status = status
	db, err = crClient.Update(db)
	if err != nil {
		return err
	}
	return nil
}
