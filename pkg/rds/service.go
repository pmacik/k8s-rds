package rds

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/pmacik/k8s-rds/pkg/kube"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// create an External named service object for Kubernetes
func (k *RDS) createServiceObj(s *v1.Service, namespace string, hostname string, internalname string) *v1.Service {
	var ports []v1.ServicePort

	ports = append(ports, v1.ServicePort{
		Name:       fmt.Sprintf("pgsql"),
		Port:       int32(5432),
		TargetPort: intstr.IntOrString{IntVal: int32(5432)},
	})
	s.Spec.Type = "ExternalName"
	s.Spec.ExternalName = hostname

	s.Spec.Ports = ports
	s.Name = internalname
	s.Annotations = map[string]string{"origin": "rds"}
	s.Namespace = namespace
	return s
}

// CreateService Creates or updates a service in Kubernetes with the new information
func (k *RDS) CreateService(namespace string, hostname string, internalname string) error {

	// create a service in kubernetes that points to the AWS RDS instance
	kubectl, err := kube.Client()
	if err != nil {
		return err
	}
	serviceInterface := kubectl.CoreV1().Services(namespace)

	s, sErr := serviceInterface.Get(hostname, metav1.GetOptions{})

	create := false
	if sErr != nil {
		s = &v1.Service{}
		create = true
	}
	s = k.createServiceObj(s, namespace, hostname, internalname)
	if create {
		_, err = serviceInterface.Create(s)
	} else {
		_, err = serviceInterface.Update(s)
	}

	return err
}

func (k *RDS) DeleteService(namespace string, dbname string) error {
	kubectl, err := kube.Client()
	if err != nil {
		return err
	}
	serviceInterface := kubectl.CoreV1().Services(namespace)
	err = serviceInterface.Delete(dbname, &metav1.DeleteOptions{})
	if err != nil {
		log.Println(err)
		return errors.Wrap(err, fmt.Sprintf("delete of service %v failed in namespace %v", dbname, namespace))
	}
	return nil
}

func (k *RDS) GetSecret(namespace string, name string, key string) (string, error) {
	kubectl, err := kube.Client()
	if err != nil {
		return "", err
	}
	secret, err := kubectl.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to fetch secret %v", name))
	}
	password := secret.Data[key]
	return string(password), nil
}