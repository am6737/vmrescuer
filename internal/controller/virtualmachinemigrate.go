package controller

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "vmrescuer/api/v1"
)

type VirtualMachineInstanceMigrationInterface interface {
	Get(name, namespace string, options *metav1.GetOptions) (*v1.VirtualMachineInstanceMigration, error)
	List(opts *metav1.ListOptions) (*v1.VirtualMachineInstanceMigrationList, error)
	Create(migration *v1.VirtualMachineInstanceMigration, options *metav1.CreateOptions) (*v1.VirtualMachineInstanceMigration, error)
	Update(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.VirtualMachineInstanceMigration, err error)
	UpdateStatus(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	PatchStatus(name string, pt types.PatchType, data []byte) (result *v1.VirtualMachineInstanceMigration, err error)

	Has(key string) (bool, *v1.VirtualMachineInstanceMigration)
}

type migration struct {
	client.Client
	v1.VirtualMachineInstanceMigration
	VMI virtv1.VirtualMachineInstance
}

func NewVirtualMachineInstanceMigration(client client.Client) *migration {
	return &migration{Client: client}
}

func (m *migration) Has(key string) (bool, *v1.VirtualMachineInstanceMigration) {
	vmiml, err := m.List(&metav1.ListOptions{})
	if err != nil {
		return false, nil
	}
	for _, vmim := range vmiml.Items {
		if key == vmim.Name {
			return true, &vmim
		}
	}
	return false, nil
}

func (m *migration) Get(name, namespace string, options *metav1.GetOptions) (*v1.VirtualMachineInstanceMigration, error) {
	resp := &v1.VirtualMachineInstanceMigration{}
	//err := m.Client.Get(context.Background(), client.ObjectKey{Name: name}, resp, &client.GetOptions{})
	err := m.Client.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, resp, &client.GetOptions{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *migration) List(opts *metav1.ListOptions) (*v1.VirtualMachineInstanceMigrationList, error) {
	var migrations = &v1.VirtualMachineInstanceMigrationList{}
	err := m.Client.List(context.Background(), migrations, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	return migrations, nil
}

func (m *migration) Create(migration *v1.VirtualMachineInstanceMigration, options *metav1.CreateOptions) (*v1.VirtualMachineInstanceMigration, error) {
	var vmim = &v1.VirtualMachineInstanceMigration{}
	err := m.Client.Create(context.Background(), migration, &client.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return vmim, nil
}

func (m *migration) Update(instanceMigration *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	//TODO implement me
	panic("implement me")
}

func (m *migration) Delete(key string, options *metav1.DeleteOptions) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	c := &v1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	//err := m.Client.Get(context.Background(), client.ObjectKey{Name: name}, migration)
	//if err != nil {
	//	return err
	//}
	return m.Client.Delete(context.Background(), c, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (m *migration) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.VirtualMachineInstanceMigration, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *migration) UpdateStatus(instanceMigration *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	//TODO implement me
	panic("implement me")
}

func (m *migration) PatchStatus(name string, pt types.PatchType, data []byte) (result *v1.VirtualMachineInstanceMigration, err error) {
	//TODO implement me
	panic("implement me")
}
