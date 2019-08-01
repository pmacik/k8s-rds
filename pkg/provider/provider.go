package provider

import (
	"github.com/pmacik/k8s-rds/pkg/crd"
)

// DatabaseProvider is the interface for creating and deleting databases
// this is the main interface that should be implemented if a new provider is created
type DatabaseProvider interface {
	CreateDatabase(*crd.Database) (string, error)
	DeleteDatabase(*crd.Database) error
	ServiceProvider
}

type ServiceProvider interface {
	CreateService(namespace string, hostname string, internalname string, owner *crd.Database) error
	DeleteService(namespace string, dbname string) error
	GetSecret(namepspace string, pwname string, pwkey string) (string, error)
}
