package controller

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "vmrescuer/api/v1"
)

type VirtualMachineInstanceRescueInterface interface {
	Get(name, namespace string, options *client.GetOptions) (*v1.VirtualMachineInstanceRescue, error)
	List(opts *metav1.ListOptions) (*v1.VirtualMachineInstanceRescueList, error)
	Create(migration *v1.VirtualMachineInstanceRescue, options *client.CreateOptions) (*v1.VirtualMachineInstanceRescue, error)
	Update(*v1.VirtualMachineInstanceRescue) (*v1.VirtualMachineInstanceRescue, error)
	Delete(name, namespace string, options *client.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.VirtualMachineInstanceRescue, err error)
	UpdateStatus(*v1.VirtualMachineInstanceRescue) (*v1.VirtualMachineInstanceRescue, error)
	PatchStatus(name string, pt types.PatchType, data []byte) (result *v1.VirtualMachineInstanceRescue, err error)
	IsEligible(key string) (*v1.VirtualMachineInstanceRescue, bool)
}

type migration struct {
	client.Client
	v1.VirtualMachineInstanceRescue
	VMI virtv1.VirtualMachineInstance
}

func NewVirtualMachineInstanceMigration(client client.Client) *migration {
	return &migration{Client: client}
}

func (m *migration) IsEligible(name string) (*v1.VirtualMachineInstanceRescue, bool) {
	vmiml, err := m.List(&metav1.ListOptions{})
	if err != nil {
		return nil, false
	}
	for _, vmim := range vmiml.Items {
		if name == vmim.Status.VMI && (vmim.Status.Phase == v1.MigrationQueuing || vmim.Status.Phase == v1.MigrationRunning) {
			return &vmim, false
		}
	}
	return nil, true
}

func (m *migration) Get(name, namespace string, options *client.GetOptions) (*v1.VirtualMachineInstanceRescue, error) {
	resp := &v1.VirtualMachineInstanceRescue{}
	err := m.Client.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, resp, &client.GetOptions{})
	//err := m.Client.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, resp, &client.GetOptions{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *migration) List(opts *metav1.ListOptions) (*v1.VirtualMachineInstanceRescueList, error) {
	var migrations = &v1.VirtualMachineInstanceRescueList{}
	err := m.Client.List(context.Background(), migrations, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	return migrations, nil
}

func (m *migration) Create(migration *v1.VirtualMachineInstanceRescue, options *client.CreateOptions) (*v1.VirtualMachineInstanceRescue, error) {
	err := m.Client.Create(context.Background(), migration, &client.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return migration, nil
}

func (m *migration) Update(instanceMigration *v1.VirtualMachineInstanceRescue) (*v1.VirtualMachineInstanceRescue, error) {
	// Call the Update method of the client to update the resource
	err := m.Client.Update(context.Background(), instanceMigration, &client.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (m *migration) Delete(name, namespace string, options *client.DeleteOptions) error {
	c := &v1.VirtualMachineInstanceRescue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return m.Client.Delete(context.Background(), c, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (m *migration) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.VirtualMachineInstanceRescue, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *migration) UpdateStatus(instanceMigration *v1.VirtualMachineInstanceRescue) (*v1.VirtualMachineInstanceRescue, error) {
	return instanceMigration, m.Client.Status().Update(context.Background(), instanceMigration)
}

func (m *migration) PatchStatus(name string, pt types.PatchType, data []byte) (result *v1.VirtualMachineInstanceRescue, err error) {
	//TODO implement me
	panic("implement me")
}
