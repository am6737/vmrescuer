package controller

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	monitorv1 "vmrescuer/api/v1"
)

type VirtualMachineInstanceRescueInterface interface {
	Get(name, namespace string, options *client.GetOptions) (*monitorv1.VirtualMachineInstanceRescue, error)
	List(opts *metav1.ListOptions) (*monitorv1.VirtualMachineInstanceRescueList, error)
	Create(migration *monitorv1.VirtualMachineInstanceRescue, options *client.CreateOptions) (*monitorv1.VirtualMachineInstanceRescue, error)
	Update(*monitorv1.VirtualMachineInstanceRescue) (*monitorv1.VirtualMachineInstanceRescue, error)
	Delete(name, namespace string, options *client.DeleteOptions) error
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *monitorv1.VirtualMachineInstanceRescue, err error)
	UpdateStatus(*monitorv1.VirtualMachineInstanceRescue) (*monitorv1.VirtualMachineInstanceRescue, error)
	PatchStatus(name string, pt types.PatchType, data []byte) (result *monitorv1.VirtualMachineInstanceRescue, err error)
	IsEligible(key string) (*monitorv1.VirtualMachineInstanceRescue, bool)
}

type migration struct {
	client.Client
	monitorv1.VirtualMachineInstanceRescue
	VMI virtv1.VirtualMachineInstance
}

func NewVirtualMachineInstanceMigration(client client.Client) *migration {
	return &migration{Client: client}
}

func (m *migration) IsEligible(name string) (*monitorv1.VirtualMachineInstanceRescue, bool) {
	vmiml, err := m.List(&metav1.ListOptions{})
	if err != nil {
		return nil, false
	}
	for _, vmim := range vmiml.Items {
		if name == vmim.Spec.VMI && (vmim.Status.Phase == monitorv1.MigrationQueuing || vmim.Status.Phase == monitorv1.MigrationRunning) {
			return &vmim, false
		}
	}
	return nil, true
}

func (m *migration) Get(name, namespace string, options *client.GetOptions) (*monitorv1.VirtualMachineInstanceRescue, error) {
	resp := &monitorv1.VirtualMachineInstanceRescue{}
	err := m.Client.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, resp, &client.GetOptions{})
	//err := m.Client.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, resp, &client.GetOptions{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *migration) List(opts *metav1.ListOptions) (*monitorv1.VirtualMachineInstanceRescueList, error) {
	var migrations = &monitorv1.VirtualMachineInstanceRescueList{}
	err := m.Client.List(context.Background(), migrations, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	return migrations, nil
}

func (m *migration) Create(migration *monitorv1.VirtualMachineInstanceRescue, options *client.CreateOptions) (*monitorv1.VirtualMachineInstanceRescue, error) {
	err := m.Client.Create(context.Background(), migration, &client.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return migration, nil
}

func (m *migration) Update(instanceMigration *monitorv1.VirtualMachineInstanceRescue) (*monitorv1.VirtualMachineInstanceRescue, error) {
	// Call the Update method of the client to update the resource
	err := m.Client.Update(context.Background(), instanceMigration, &client.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (m *migration) Delete(name, namespace string, options *client.DeleteOptions) error {
	c := &monitorv1.VirtualMachineInstanceRescue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return m.Client.Delete(context.Background(), c, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func (m *migration) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *monitorv1.VirtualMachineInstanceRescue, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *migration) UpdateStatus(instanceMigration *monitorv1.VirtualMachineInstanceRescue) (*monitorv1.VirtualMachineInstanceRescue, error) {
	return instanceMigration, m.Client.Status().Update(context.Background(), instanceMigration)
}

func (m *migration) PatchStatus(name string, pt types.PatchType, data []byte) (result *monitorv1.VirtualMachineInstanceRescue, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *VirtualMachineNodeWatcherReconciler) vmirDeleteHandler(e event.DeleteEvent, q workqueue.RateLimitingInterface) {

	vmir, ok := e.Object.(*monitorv1.VirtualMachineInstanceRescue)
	if !ok {
		r.Log.Error(fmt.Errorf("unexpected object type for deleted object: %T", e.Object), "failed to get deleted VirtualMachineInstanceRescue object")
		return
	}

	key := fmt.Sprintf("%s/%s", vmir.Namespace, vmir.Name)
	r.tw.RemoveTimer(key)

	r.Log.Info(fmt.Sprintf("Delete VirtualMachineInstance %s in migration queue", key))
}

func (r *VirtualMachineNodeWatcherReconciler) vmirCreateHandler(e event.CreateEvent, q workqueue.RateLimitingInterface) {

	vmir, ok := e.Object.(*monitorv1.VirtualMachineInstanceRescue)
	if !ok {
		r.Log.Error(fmt.Errorf("unexpected object type for created object: %T", e.Object), "failed to get created VirtualMachineInstanceRescue object")
		return
	}

	key := fmt.Sprintf("%s/%s", vmir.Namespace, vmir.Name)
	// 添加迁移任务到延迟队列 到达设置的阈值时间时启动虚拟机迁移
	r.tw.AddTimer(r.interval, key, map[string]string{"key": key})

	r.Log.Info(fmt.Sprintf("Add VirtualMachineInstance %s in migration queue", key))
}

func (r *VirtualMachineNodeWatcherReconciler) vmirUpdateHandler(e event.UpdateEvent, q workqueue.RateLimitingInterface) {

	oldVmim, ok := e.ObjectOld.(*monitorv1.VirtualMachineInstanceRescue)
	if !ok {
		r.Log.Info("vmirUpdateHandler: Old object is not a VirtualMachineInstanceRescue resource")
		return
	}

	newVmim, ok := e.ObjectNew.(*monitorv1.VirtualMachineInstanceRescue)
	if !ok {
		r.Log.Info("vmirUpdateHandler: New object is not a VirtualMachineInstanceRescue resource")
		return
	}

	// 判断状态是否从其他状态变为Cancel
	if oldVmim.Status.Phase != monitorv1.MigrationCancel && newVmim.Status.Phase == monitorv1.MigrationCancel {
		// 取消延迟队列中的任务
		key := fmt.Sprintf("%s/%s", newVmim.Namespace, newVmim.Name)
		r.tw.RemoveTimer(key)
		r.Log.Info(fmt.Sprintf("Canceled migration task for VirtualMachineInstanceRescue %s", key))
	}
}
