package controller

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	monitorv1 "vmrescuer/api/v1"
)

type VirtualMachineMigration struct {
	monitorv1.VirtualMachineMigration
	client.Client
}

func NewVirtualMachineMigration(client client.Client) *VirtualMachineMigration {
	return &VirtualMachineMigration{Client: client}
}

func (c *VirtualMachineMigration) Create(ctx context.Context, migration *monitorv1.VirtualMachineMigration) error {
	err := c.Client.Create(ctx, migration)
	return err
}

func (c *VirtualMachineMigration) Update(migration *monitorv1.VirtualMachineMigration) error {
	err := c.Client.Update(context.Background(), migration)
	return err
}

func (c *VirtualMachineMigration) Delete(name string) error {
	migration := &VirtualMachineMigration{}
	err := c.Client.Get(context.Background(), client.ObjectKey{Name: name}, migration)
	if err != nil {
		return err
	}
	err = c.Client.Delete(context.Background(), migration)
	return err
}

func (c *VirtualMachineMigration) Get(name string) (*VirtualMachineMigration, error) {
	migration := &VirtualMachineMigration{}
	err := c.Client.Get(context.Background(), client.ObjectKey{Name: name}, migration)
	if err != nil {
		return nil, err
	}
	return migration, nil
}

func (c *VirtualMachineMigration) List() ([]VirtualMachineMigration, error) {
	var migrations []VirtualMachineMigration
	err := c.Client.List(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return migrations, nil
}
