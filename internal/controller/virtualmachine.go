package controller

import (
	"context"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

// VirtualMachineInterface 定义虚拟机相关的操作接口
type VirtualMachineInterface interface {
	Get(ctx context.Context, namespace, name string) (*virtv1.VirtualMachineInstance, error)
	List(ctx context.Context) (*virtv1.VirtualMachineInstanceList, error)
	IsMigrate(ctx context.Context, vmi *virtv1.VirtualMachineInstance) bool
	Migrate(ctx context.Context, name, namespace string) error
	IsMigrating(ctx context.Context, name, namespace string) (bool, error)
}

type Option func(*VirtualMachine)

type VirtualMachine struct {
	virtClient kubecli.KubevirtClient
}

// NewDefaultVirtualMachine 创建一个带有默认 Kubevirt 客户端的 VirtualMachine 实例
func NewDefaultVirtualMachine() *VirtualMachine {
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(kubecli.DefaultClientConfig(&pflag.FlagSet{}))
	if err != nil {
		panic(err)
	}
	return &VirtualMachine{virtClient: virtClient}
}

// NewVirtualMachine 创建一个带有指定 Kubevirt 客户端的 VirtualMachine 实例
func NewVirtualMachine(virtClient kubecli.KubevirtClient) *VirtualMachine {
	return &VirtualMachine{virtClient: virtClient}
}

// Get 获取指定虚拟机 VirtualMachineInstance 资源
func (v *VirtualMachine) Get(ctx context.Context, namespace, name string) (*virtv1.VirtualMachineInstance, error) {
	vm, err := v.virtClient.VirtualMachineInstance(namespace).Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// List 获取所有 VirtualMachineInstance 资源
func (v *VirtualMachine) List(ctx context.Context) (*virtv1.VirtualMachineInstanceList, error) {
	vmiList, err := v.virtClient.VirtualMachineInstance(metav1.NamespaceAll).List(ctx, &metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return vmiList, nil
}

// IsMigrate 检查给定的 VirtualMachineInstance 是否可迁移
func (v *VirtualMachine) IsMigrate(ctx context.Context, vmi *virtv1.VirtualMachineInstance) bool {
	return vmi.IsMigratable()
}

// Migrate 迁移指定的 VirtualMachineInstance 到另一个节点
func (v *VirtualMachine) Migrate(ctx context.Context, name, namespace string) error {
	return v.virtClient.VirtualMachine(namespace).Migrate(ctx, name, &virtv1.MigrateOptions{})
}

// IsMigrating 检查给定的 VirtualMachineInstance 是否正在进行迁移
func (v *VirtualMachine) IsMigrating(ctx context.Context, name, namespace string) (bool, error) {
	migrationList, err := v.virtClient.VirtualMachineInstanceMigration(metav1.NamespaceAll).List(&metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, migration := range migrationList.Items {
		if migration.Spec.VMIName == name && migration.Namespace == namespace {
			if migration.Status.Phase != virtv1.MigrationFailed && migration.Status.Phase != virtv1.MigrationSucceeded {
				return true, nil
			}
		}
	}
	return false, nil
}
