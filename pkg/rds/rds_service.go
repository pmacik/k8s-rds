package rds

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/pmacik/k8s-rds/pkg/crd"
	"github.com/pmacik/k8s-rds/pkg/kube"
	"github.com/pmacik/k8s-rds/pkg/provider"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// create an External named service object for Kubernetes
func (k *RDS) createServiceObj(s *v1.Service, namespace string, dbEndpoint *provider.DBEndpoint, internalname string) *v1.Service {
	var ports []v1.ServicePort

	dbPort := dbEndpoint.Port
	ports = append(ports, v1.ServicePort{
		Name:       fmt.Sprintf("pgsql"),
		Port:       int32(dbPort),
		TargetPort: intstr.IntOrString{IntVal: int32(dbPort)},
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
func (k *RDS) CreateService(namespace string, dbEndpoint *provider.DBEndpoint, internalname string, owner *crd.Database) (*v1.Service, error) {

	// create a service in kubernetes that points to the AWS RDS instance
	kubectl, err := kube.Client()
	if err != nil {
		return nil, err
	}
	lbs := map[string]string{
		"app": owner.GetName(),
	}
	serviceInterface := kubectl.CoreV1().Services(namespace)
	log.Printf("DBEndpoint=%v", dbEndpoint)

	s, sErr := serviceInterface.Get(dbEndpoint.Hostname, metav1.GetOptions{})
	create := false
	if sErr != nil {
		s = &v1.Service{}
		s = k.createServiceObj(s, namespace, dbEndpoint, internalname)
		s.SetLabels(lbs)
		ownerRef := metav1.OwnerReference{
			APIVersion: crd.CRDVersion,
			Kind:       crd.CRDKind,
			Name:       owner.GetName(),
			UID:        owner.GetUID(),
		}
		s.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
		create = true
	}

	if create {
		_, err = serviceInterface.Create(s)
	} else {
		_, err = serviceInterface.Update(s)
	}

	return s, err
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

func (k *RDS) GetSecret(namespace string, name string) (*v1.Secret, error) {
	kubectl, err := kube.Client()
	if err != nil {
		return nil, err
	}
	secret, err := kubectl.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to fetch secret %v", name))
	}
	return secret, nil
}
