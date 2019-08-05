package client

import (
	"github.com/pmacik/k8s-rds/pkg/crd"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// This file implement all the (CRUD) client methods we need to access our CR objects

func NewCRClient(restClient *rest.RESTClient, scheme *runtime.Scheme, namespace string) *CRClient {
	return &CRClient{restClient: restClient, ns: namespace, plural: crd.CRDPlural,
		codec: runtime.NewParameterCodec(scheme)}
}

type CRClient struct {
	restClient *rest.RESTClient
	ns         string
	plural     string
	codec      runtime.ParameterCodec
}

func (crClient *CRClient) Create(obj *crd.Database) (*crd.Database, error) {
	var result crd.Database
	err := crClient.restClient.Post().
		Namespace(crClient.ns).Resource(crClient.plural).
		Body(obj).Do().Into(&result)
	return &result, err
}

func (crClient *CRClient) Update(obj *crd.Database) (*crd.Database, error) {
	var result crd.Database
	err := crClient.restClient.Put().
		Namespace(crClient.ns).Resource(crClient.plural).Name(obj.Name).
		Body(obj).Do().Into(&result)
	return &result, err
}

func (crClient *CRClient) Delete(name string, options *meta_v1.DeleteOptions) error {
	return crClient.restClient.Delete().
		Namespace(crClient.ns).Resource(crClient.plural).
		Name(name).Body(options).Do().
		Error()
}

func (crClient *CRClient) Get(name string) (*crd.Database, error) {
	var result crd.Database
	err := crClient.restClient.Get().
		Namespace(crClient.ns).Resource(crClient.plural).
		Name(name).Do().Into(&result)
	return &result, err
}

func (crClient *CRClient) List(opts meta_v1.ListOptions) (*crd.DatabaseList, error) {
	var result crd.DatabaseList
	err := crClient.restClient.Get().
		Namespace(crClient.ns).Resource(crClient.plural).
		VersionedParams(&opts, crClient.codec).
		Do().Into(&result)
	return &result, err
}

// Create a new List watch for our CRD
func (crClient *CRClient) NewListWatch() *cache.ListWatch {
	return cache.NewListWatchFromClient(crClient.restClient, crClient.plural, crClient.ns, fields.Everything())
}
