package kube

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/pmacik/k8s-rds/pkg/crd"
	"github.com/pmacik/k8s-rds/pkg/provider"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type Kube struct {
	Client *kubernetes.Clientset
}

// create an External named service object for Kubernetes
func (k *Kube) createServiceObj(s *v1.Service, namespace string, dbEndpoint *provider.DBEndpoint, internalname string) *v1.Service {
	var ports []v1.ServicePort
	port := dbEndpoint.Port
	ports = append(ports, v1.ServicePort{
		Name:       fmt.Sprintf("pgsql"),
		Port:       int32(port),
		TargetPort: intstr.IntOrString{IntVal: int32(port)},
	})
	s.Spec.Type = "ExternalName"
	s.Spec.ExternalName = dbEndpoint.Hostname

	s.Spec.Ports = ports
	s.Name = internalname
	s.Annotations = map[string]string{"origin": "rds"}
	s.Namespace = namespace
	return s
}

// CreateService Creates or updates a service in Kubernetes with the new information
func (k *Kube) CreateService(namespace string, dbEndpoint *provider.DBEndpoint, internalname string, owner *crd.Database) (*v1.Service, error) {

	// create a service in kubernetes that points to the AWS RDS instance
	serviceInterface := k.Client.CoreV1().Services(namespace)
	hostname := dbEndpoint.Hostname
	s, sErr := serviceInterface.Get(hostname, metav1.GetOptions{})

	create := false
	if sErr != nil {
		s = &v1.Service{}
		create = true
	}
	s = k.createServiceObj(s, namespace, dbEndpoint, internalname)
	var err error
	if create {
		_, err = serviceInterface.Create(s)
	} else {
		_, err = serviceInterface.Update(s)
	}

	return s, err
}

func (k *Kube) DeleteService(namespace string, dbname string) error {
	serviceInterface := k.Client.CoreV1().Services(namespace)
	err := serviceInterface.Delete(dbname, &metav1.DeleteOptions{})
	if err != nil {
		log.Println(err)
		return errors.Wrap(err, fmt.Sprintf("delete of service %v failed in namespace %v", dbname, namespace))
	}
	return nil
}

func (k *Kube) GetSecret(namespace string, name string, key string) (string, error) {
	secret, err := k.Client.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to fetch secret %v", name))
	}
	password := secret.Data[key]
	return string(password), nil
}
