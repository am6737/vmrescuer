package controller

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "vmrescuer/api/v1"
)

type VirtualMachineInstanceMigrationInterface interface {
	Get(name string, options *metav1.GetOptions) (*v1.VirtualMachineInstanceMigration, error)
	List(opts *metav1.ListOptions) (*v1.VirtualMachineInstanceMigrationList, error)
	Create(migration *v1.VirtualMachineInstanceMigration, options *metav1.CreateOptions) (*v1.VirtualMachineInstanceMigration, error)
	Update(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.VirtualMachineInstanceMigration, err error)
	UpdateStatus(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	PatchStatus(name string, pt types.PatchType, data []byte) (result *v1.VirtualMachineInstanceMigration, err error)
}

type migration struct {
	client.Client
	v1.VirtualMachineInstanceMigration
	VMI virtv1.VirtualMachineInstance
}

func NewVirtualMachineInstanceMigration(client client.Client) *migration {
	return &migration{Client: client}
}

func (m *migration) Get(name string, options *metav1.GetOptions) (*v1.VirtualMachineInstanceMigration, error) {
	migration := &v1.VirtualMachineInstanceMigration{}
	err := m.Client.Get(context.Background(), client.ObjectKey{Name: name}, migration, &client.GetOptions{})
	if err != nil {
		return nil, err
	}
	return migration, nil
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

func (m *migration) Delete(name string, options *metav1.DeleteOptions) error {
	migration := &migration{}
	err := m.Client.Get(context.Background(), client.ObjectKey{Name: name}, migration)
	if err != nil {
		return err
	}
	err = m.Client.Delete(context.Background(), migration, &client.DeleteOptions{})
	return err
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
