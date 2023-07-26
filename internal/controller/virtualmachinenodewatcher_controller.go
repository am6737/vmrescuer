/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	virtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sync"
	"time"
	monitorv1 "vmrescuer/api/v1"
)

// VirtualMachineNodeWatcherReconciler reconciles a VirtualMachineNodeWatcher object
type VirtualMachineNodeWatcherReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	recorder  record.EventRecorder
	workqueue workqueue.RateLimitingInterface

	vm   VirtualMachineInterface
	node NodeWatcherInterface
	//vmm  *VirtualMachineMigration

	ctx          context.Context
	cancel       context.CancelFunc
	Log          logr.Logger
	ticker       *time.Ticker
	interval     time.Duration
	migratingVMs sync.Map

	run             bool
	runWorkerStopCh chan struct{}
}

type VirtualMachineInstanceMigrationPhase string

const (
	MigrationPending   VirtualMachineInstanceMigrationPhase = "Pending"
	MigrationRunning   VirtualMachineInstanceMigrationPhase = "Running"
	MigrationSucceeded VirtualMachineInstanceMigrationPhase = "Succeeded"
	MigrationFailed    VirtualMachineInstanceMigrationPhase = "Failed"
	MigrationCancel    VirtualMachineInstanceMigrationPhase = "Cancel"
)

type migrationVM struct {
	VMI   virtv1.VirtualMachineInstance
	Node  string
	Phase VirtualMachineInstanceMigrationPhase
	//VMM   monitorv1.VirtualMachineMigration
}

func NewVirtualMachineNodeWatcherReconciler(mgr ctrl.Manager) *VirtualMachineNodeWatcherReconciler {
	ctx, cancel := context.WithCancel(context.Background()) // 创建上下文和取消函数
	return &VirtualMachineNodeWatcherReconciler{
		Client:    mgr.GetClient(),
		Log:       mgr.GetLogger(),
		Scheme:    mgr.GetScheme(),
		workqueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:  mgr.GetEventRecorderFor("VirtualMachineNodeWatcher"),
		vm:        NewDefaultVirtualMachine(),
		//vmm:             NewVirtualMachineMigration(mgr.GetClient()),
		node:            NewNodeWatcher(mgr.GetClient()),
		ticker:          time.NewTicker(1 * time.Minute),
		interval:        5,
		ctx:             ctx,
		cancel:          cancel,
		runWorkerStopCh: make(chan struct{}),
	}
}

//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinenodewatchers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinenodewatchers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinenodewatchers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VirtualMachineNodeWatcher object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *VirtualMachineNodeWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("virtualmachinenodewatcher", req.NamespacedName)
	// 获取 VirtualMachineNodeWatcher 对象
	vmnw := &monitorv1.VirtualMachineNodeWatcher{}
	if err := r.Get(ctx, req.NamespacedName, vmnw); err != nil {
		// 处理获取对象失败的情况
		return ctrl.Result{}, err
	}

	interval, err := time.ParseDuration(vmnw.Spec.Interval)
	if err != nil {
		log.Error(err, "failed to parse interval")
		return ctrl.Result{}, fmt.Errorf("failed to parse interval: %v", err)
	}
	r.interval = interval
	// 处理对象创建和更新时的逻辑
	if vmnw.ObjectMeta.DeletionTimestamp.IsZero() {
		if vmnw.Spec.Enable && !r.run {
			// 如果 Spec.Enable 为 true，并且之前未启动 worker
			if r.ticker != nil {
				r.ticker.Reset(r.interval)
			} else {
				r.ticker = time.NewTicker(r.interval)
			}
			r.run = true
			go r.runWorker(r.ctx)
		} else if !vmnw.Spec.Enable && r.run {
			// 如果 Spec.Enable 为 false，并且之前正在运行 worker
			r.run = false
			r.runWorkerStopCh <- struct{}{}
			r.ticker.Stop()
		} else if vmnw.Spec.Enable {
			// 如果 Spec.Enable 为 true，并且之前已经启动 worker，则只重置定时器
			r.ticker.Reset(r.interval)
		}
	} else {
		r.run = false
		r.runWorkerStopCh <- struct{}{}
		if r.ticker != nil {
			r.ticker.Stop()
		}
	}

	// 其他 Reconcile 逻辑...
	return ctrl.Result{}, nil
}

// handleVirtualMachineMigration will handle the creation and deletion of VirtualMachineMigration resources
func (r *VirtualMachineNodeWatcherReconciler) handleVirtualMachineMigration(vmMigration *monitorv1.VirtualMachineMigration) error {
	// Check if the VirtualMachineMigration is being created or deleted
	if vmMigration.DeletionTimestamp != nil {
		// Handle deletion logic
		r.migratingVMs.Delete(vmMigration.Name)
		r.Log.Info(fmt.Sprintf("Removed VM %s from migration queue.", vmMigration.Name))
	} else {
		// Handle creation logic
		// Extract relevant information from the VirtualMachineMigration resource and add it to migratingVMs
		// For example:
		migrationInfo := &monitorv1.VirtualMachineMigrationStatus{
			Name:   vmMigration.Status.Name,
			Status: vmMigration.Status.Status,
			Node:   vmMigration.Status.Node,
			//MigrationTime: vmMigration.Status.MigrationTime.Time,
		}
		r.migratingVMs.Store(vmMigration.Name, migrationInfo)
		r.Log.Info(fmt.Sprintf("Added VM %s to migration queue.", vmMigration.Name))
	}
	return nil
}

func (r *VirtualMachineNodeWatcherReconciler) runWorker(ctx context.Context) {
	defer r.ticker.Stop()
	go func() {
		for {
			select {
			case <-r.ticker.C:
				r.syncQueue()
			case <-r.runWorkerStopCh:
				// 收到信号，停止处理循环
				r.Log.Info("收到关闭通道信号 结束 runWorker")
				r.clean()
				return
			case <-r.ctx.Done():
				// 上下文已取消，停止处理循环
				r.Log.Info("结束 runWorker")
				return
			}
		}
	}()
	for r.processQueue(ctx) {
	}
}

// syncQueue 方法用于同步虚拟机迁移队列 定时将迁移的虚拟机列表加入工作队列
func (r *VirtualMachineNodeWatcherReconciler) syncQueue() {
	defer r.recorder.Event(&monitorv1.VirtualMachineNodeWatcher{}, corev1.EventTypeNormal, "SyncComplete", "Sync of virtual machine migration queue complete")

	r.migratingVMs.Range(func(key, value interface{}) bool {
		vmName := key.(string)
		r.workqueue.Add(vmName)
		return true
	})

	r.clean()
}

func (r *VirtualMachineNodeWatcherReconciler) addMigration(name string, mvm *migrationVM) {
	key := fmt.Sprintf("%s/%s", mvm.VMI.Namespace, name)

	// 是否已经存在迁移的列表中
	if _, ok := r.migratingVMs.Load(key); ok {
		return
	}
	//migration := &monitorv1.VirtualMachineMigration{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      name,
	//		Namespace: mvm.VMI.Namespace, // 如果需要指定 Namespace，根据实际情况设置
	//	},
	//	Status: monitorv1.VirtualMachineMigrationStatus{
	//		Name: mvm.VMI.Name,
	//		//Status:        mvm.Status,
	//		Node:          mvm.Node,
	//		MigrationTime: metav1.Now(),
	//	},
	//}
	//mvm.VMM = *migration
	//// 创建VirtualMachineMigration资源
	//err := r.vmm.Create(context.Background(), migration)
	//if err != nil {
	//	r.Log.Error(err, "Create VirtualMachineMigration Resources")
	//	//return
	//}
<<<<<<< HEAD
	r.Log.Info(fmt.Sprintf("Add a virtual machine instance %s In Migration Queue", key))
=======
	r.Log.Info(fmt.Sprintf("添加虚拟机实例 %s 入迁移队列", key))
>>>>>>> 5bcb94f... 添加自定义资源的rbac注解
	r.migratingVMs.Store(key, mvm)
}

func (r *VirtualMachineNodeWatcherReconciler) processQueue(ctx context.Context) bool {
	obj, shutdown := r.workqueue.Get()
	if shutdown {
		return false
	}

	//// 无论迁移虚拟机是否成功清理
	//clean := func(key string, obj interface{}) {
	//	r.migratingVMs.Delete(key)
	//	r.workqueue.Forget(obj)
	//}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer r.workqueue.Done(obj)
		var key string
		var ok bool
		//defer clean(key, obj)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			r.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := r.syncHandler(ctx, key); err != nil {
			r.Log.Error(err, fmt.Sprintf("虚拟机实例%s 迁移失败重新加入迁移队列", key))
			// Put the item back on the workqueue to handle any transient errors.
			r.workqueue.AddRateLimited(key)
			r.workqueue.Forget(obj)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		r.migratingVMs.Delete(key)
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		r.workqueue.Forget(obj)
		r.Log.Info(fmt.Sprintf("Successfully synced %s resourceName", key))
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (r *VirtualMachineNodeWatcherReconciler) syncHandler(ctx context.Context, key string) error {
	if mvm, ok := r.migratingVMs.Load(key); ok {
		if mvm.(*migrationVM).Phase == MigrationCancel {
			r.Log.Info(fmt.Sprintf("Resume node status to cancel migration %s", key))
			//err := r.vmm.Delete(mvm.(*migrationVM).VMM.Name)
			//if err != nil {
			//	r.Log.Error(err, "Delete VirtualMachineMigration Resource failure")
			//	return err
			//}
			r.migratingVMs.Delete(key)
			return nil
		}
		//defer func() {
		//	err := r.vmm.Delete(mvm.(*migrationVM).VMM.Name)
		//	if err != nil {
		//		r.Log.Error(err, "Delete VirtualMachineMigration Resource failure")
		//	}
		//}()
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// 根据 Namespace 和 Name 获取虚拟机实例对象
	vmi, err := r.vm.Get(ctx, namespace, name)
	if err != nil {
		return err
	}

	// 检查虚拟机是否正在迁移
	isMigrating, err := r.vm.IsMigrating(ctx, vmi)
	if err != nil {
		return err
	}

	// 如果虚拟机正在迁移，则不进行迁移操作，直接返回
	if isMigrating {
		r.Log.Info(fmt.Sprintf("VMI %s is Migrating", vmi.Name))
		return nil
	}

	// 使用删除pod来替代使用kubevirt客户端的迁移
	// 执行虚拟机实例迁移
	ok, err := r.node.Migrate(ctx, vmi)
	if err != nil {
		return err
	}
	if ok {
		r.Log.Info(fmt.Sprintf("Start Virtual Machine Migration Name:%s Namespace:%s Status:%s Node:%s",
			vmi.Name,
			vmi.Namespace,
			vmi.Status.Phase,
			vmi.Status.NodeName,
		))
	}

	//// 执行虚拟机实例迁移
	//if err = r.vm.Migrate(ctx, vmi.Name, namespace); err != nil {
	//	return err
	//}

	// 返回 nil 表示同步成功
	return nil
}

// clean 清空迁移的虚拟机列表
func (r *VirtualMachineNodeWatcherReconciler) clean() {
	r.migratingVMs.Range(func(key, value interface{}) bool {
		//err := r.vmm.Delete(value.(*migrationVM).VMM.Name)
		//if err != nil {
		//	r.Log.Error(err, "Delete VirtualMachineMigration Resource failure")
		//	return false
		//}
		r.migratingVMs.Delete(key)
		return true
	})
}

// syncVMsToMigrate 获取故障节点需要迁移的虚拟机
func (r *VirtualMachineNodeWatcherReconciler) syncVMsToMigrate(ctx context.Context) error {
	var nodes []corev1.Node
	func() {
		list := &corev1.NodeList{}
		if err := r.List(ctx, list); err != nil {
			r.Log.Error(err, "Failed to get node list")
			return
		}
		for _, node := range list.Items {
			// 检查节点是否无法调度或者不处于 Ready 状态
			if !r.node.IsNodeReady(&node) {
				nodes = append(nodes, node)
			}
		}
	}()
	if len(nodes) == 0 {
		r.clean()
		return nil
	}

	// 内部函数用于获取运行虚拟机节点
	runningVirtualMachineNodes := func(ctx context.Context) (map[string][]virtv1.VirtualMachineInstance, error) {
		// 获取所有 VirtualMachineInstance 资源
		vmiList, err := r.vm.List(ctx)
		if err != nil {
			return nil, err
		}

		// 创建一个按节点名称存储运行中 VirtualMachineInstances 的映射
		runningNodes := make(map[string][]virtv1.VirtualMachineInstance)

		// 过滤运行中的 VirtualMachineInstances，并按节点名称分组
		for _, vmi := range vmiList.Items {
			if vmi.Status.Phase == virtv1.Running {
				runningNodes[vmi.Status.NodeName] = append(runningNodes[vmi.Status.NodeName], vmi)
			}
		}
		return runningNodes, nil
	}

	UnhealthyVMIS, err := func(ctx context.Context) ([]virtv1.VirtualMachineInstance, error) {
		runningNodes, err := runningVirtualMachineNodes(ctx)
		if err != nil {
			return nil, err
		}

		//for k, _ := range runningNodes {
		//	r.Log.Info(fmt.Sprintf("运行虚拟机的节点 %s", k))
		//}

		// 定义一个列表来存储运行在不健康节点上的虚拟机实例
		var vs []virtv1.VirtualMachineInstance

		// 过滤运行在不健康节点上的虚拟机实例
		for _, node := range nodes {
			// 检查节点是否无法调度或者不处于 Ready 状态
			if !r.node.IsNodeReady(&node) {
				// 获取运行在此节点上的虚拟机实例
				if vmiList, ok := runningNodes[node.Name]; ok {
					vs = append(vs, vmiList...)
				}
			}
		}
		return vs, nil
	}(ctx)

	if err != nil {
		return err
	}

	if len(UnhealthyVMIS) == 0 {
		return nil
	}

	// 遍历不健康的虚拟机实例列表 UnhealthyVMIS，其中包含运行在不健康节点上的虚拟机实例。
	// 对于每个虚拟机实例 vmi，我们首先调用 r.vm.IsMigrating 方法来检查虚拟机是否正在进行迁移。
	// 如果虚拟机没有在进行迁移并且可以进行迁移（即满足迁移条件），则调用 r.addMigration 方法将虚拟机添加到迁移队列中，以进行后续的迁移操作。
	for _, vmi := range UnhealthyVMIS {
		ok, err := r.vm.IsMigrating(ctx, &vmi)
		if err != nil {
			r.Log.Error(err, "r.vm.IsMigrating")
			continue
		}
		if !ok && vmi.IsMigratable() {
			//r.Log.Info(fmt.Sprintf("需要迁移的虚拟机实例 %s namespace %s node %s", vmi.Name, vmi.Namespace, vmi.Status.NodeName))
			r.addMigration(vmi.Name, &migrationVM{
				VMI:   vmi,
				Node:  vmi.Status.NodeName,
				Phase: MigrationPending,
			})
		}
	}

	return nil
}

func (r *VirtualMachineNodeWatcherReconciler) nodeUpdateHandler(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	//_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()

	if !r.run {
		return
	}

	newNode, ok := e.ObjectNew.(*corev1.Node)
	if !ok {
		fmt.Println(fmt.Errorf("unexpected object type for new object: %T", e.ObjectNew), "failed to get new node object")
		return
	}

	if r.node.IsNodeReady(newNode) {
		// 如果节点恢复健康状态了 将迁移队列里有关节点的虚拟机删除
		// 检查与节点相关的虚拟机并从迁移队列中删除
		r.migratingVMs.Range(func(key, value interface{}) bool {
			vmName, mvm := key.(string), value.(*migrationVM)
			if mvm.Node == newNode.Name {
				mvm.Phase = MigrationCancel
				r.migratingVMs.Store(vmName, mvm)
				r.Log.Info(fmt.Sprintf("Node Recovery Removed %s From Migration Queue", vmName))
			}
			return true
		})
	}

	if err := r.syncVMsToMigrate(r.ctx); err != nil {
		r.Log.Error(err, "Failed to get the list of virtual machines to be migrated")
	}
}

// SetupWithManager 注册 Informer 监听节点的变化
func (r *VirtualMachineNodeWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1.VirtualMachineNodeWatcher{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, handler.Funcs{UpdateFunc: r.nodeUpdateHandler}).
		Complete(r)
}
