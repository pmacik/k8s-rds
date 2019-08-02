package provider

import (
	"github.com/pmacik/k8s-rds/pkg/crd"
	v1 "k8s.io/api/core/v1"
)

// DatabaseProvider is the interface for creating and deleting databases
// this is the main interface that should be implemented if a new provider is created
type DatabaseProvider interface {
	CreateDatabase(*crd.Database) (string, error)
	DeleteDatabase(*crd.Database) error
	ServiceProvider
}

type ServiceProvider interface {
	CreateService(namespace string, hostname string, internalname string, owner *crd.Database) (*v1.Service, error)
	DeleteService(namespace string, dbname string) error
	GetSecret(namepspace string, pwname string) (*v1.Secret, error)
}
