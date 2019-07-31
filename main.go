package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pmacik/k8s-rds/pkg/client"
	"github.com/pmacik/k8s-rds/pkg/crd"
	"github.com/pmacik/k8s-rds/pkg/kube"
	"github.com/pmacik/k8s-rds/pkg/local"
	"github.com/pmacik/k8s-rds/pkg/provider"
	"github.com/pmacik/k8s-rds/pkg/rds"
	"github.com/spf13/cobra"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
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
	rootCmd.PersistentFlags().StringVar(&provider, "provider", "aws", "Type of provider (aws, local)")
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
	err = crd.CreateCRD(clientset)
	if err != nil {
		panic(err)
	}

	// Create a new clientset which include our CRD schema
	crdcs, scheme, err := crd.NewClient(config)
	if err != nil {
		panic(err)
	}

	// Create a CRD client interface
	crdclient := client.CrdClient(crdcs, scheme, "")
	log.Println("Watching for database changes...")
	_, controller := cache.NewInformer(
		crdclient.NewListWatch(),
		&crd.Database{},
		time.Minute*2,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				db := obj.(*crd.Database)
				client := client.CrdClient(crdcs, scheme, db.Namespace) // add the database namespace to the client
				err = handleCreateDatabase(db, client, dbprovider)
				if err != nil {
					log.Printf("database creation failed: %v", err)
					err := updateStatus(db, crd.DatabaseStatus{Message: fmt.Sprintf("%v", err), State: Failed}, client)
					if err != nil {
						log.Printf("database CRD status update failed: %v", err)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				db := obj.(*crd.Database)
				log.Printf("deleting database: %s \n", db.Name)

				r, err := getProvider(db, dbprovider)
				if err != nil {
					log.Println(err)
					return
				}

				err = r.DeleteDatabase(db)
				if err != nil {
					log.Println(err)
				}

				err = r.DeleteService(db.Namespace, db.Name)
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

	case "local":
		r, err := local.New(db, kubectl)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	return nil, fmt.Errorf("unable to find provider for %v", dbprovider)
}

func handleCreateDatabase(db *crd.Database, crdclient *client.Crdclient, dbprovider string) error {
	if db.Status.State == "Created" {
		log.Printf("database %v already created, skipping\n", db.Name)
		return nil
	}
	// validate dbname is only alpha numeric
	err := updateStatus(db, crd.DatabaseStatus{Message: "Creating", State: "Creating"}, crdclient)
	if err != nil {
		return fmt.Errorf("database CRD status update failed: %v", err)
	}

	log.Println("trying to get kubectl")

	r, err := getProvider(db, dbprovider)
	if err != nil {
		return err
	}

	hostname, err := r.CreateDatabase(db)
	if err != nil {
		return err
	}
	log.Printf("Creating service '%v' for %v\n", db.Name, hostname)
	err = r.CreateService(db.Namespace, hostname, db.Name)
	if err != nil {
		return err
	}

	err = updateStatus(db, crd.DatabaseStatus{Message: "Created", State: "Created"}, crdclient)
	if err != nil {
		return err
	}
	log.Printf("Creation of database %v done\n", db.Name)
	return nil
}

func updateStatus(db *crd.Database, status crd.DatabaseStatus, crdclient *client.Crdclient) error {
	db, err := crdclient.Get(db.Name)
	if err != nil {
		return err
	}

	db.Status = status
	_, err = crdclient.Update(db)
	if err != nil {
		return err
	}
	return nil
}
