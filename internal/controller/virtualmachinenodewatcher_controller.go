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
	"github.com/ouqiang/timewheel"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	virtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
	vmm  VirtualMachineInstanceRescueInterface
	node NodeWatcherInterface
	tw   *timewheel.TimeWheel

	ctx          context.Context
	cancel       context.CancelFunc
	Log          logr.Logger
	cleanupTimer *time.Ticker
	interval     time.Duration

	run             bool
	runWorkerStopCh chan struct{}
}

func NewVirtualMachineNodeWatcherReconciler(mgr ctrl.Manager) *VirtualMachineNodeWatcherReconciler {
	ctx, cancel := context.WithCancel(context.Background())
	return &VirtualMachineNodeWatcherReconciler{
		Client:          mgr.GetClient(),
		Log:             mgr.GetLogger(),
		Scheme:          mgr.GetScheme(),
		workqueue:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder:        mgr.GetEventRecorderFor("VirtualMachineNodeWatcher"),
		vm:              NewDefaultVirtualMachine(),
		vmm:             NewVirtualMachineInstanceMigration(mgr.GetClient()),
		node:            NewNodeWatcher(mgr.GetClient()),
		cleanupTimer:    time.NewTicker(7 * 24 * time.Hour),
		ctx:             ctx,
		cancel:          cancel,
		runWorkerStopCh: make(chan struct{}),
	}
}

//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinenodewatchers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinenodewatchers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinenodewatchers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch
//+kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstancemigrations,verbs=get;list
//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachineinstancerescues,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachineinstancerescues/status,verbs=get;update;patch

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

	vmnw := &monitorv1.VirtualMachineNodeWatcher{}
	if err := r.Get(ctx, req.NamespacedName, vmnw); err != nil {
		return ctrl.Result{}, err
	}

	if !vmnw.ObjectMeta.DeletionTimestamp.IsZero() {
		r.close()
		return ctrl.Result{}, nil
	}

	interval, err := time.ParseDuration(vmnw.Spec.Interval)
	if err != nil {
		log.Error(err, "failed to parse interval")
		return ctrl.Result{}, fmt.Errorf("failed to parse interval: %v", err)
	}
	r.interval = interval

	clean, err := time.ParseDuration(vmnw.Spec.Clean)
	if err != nil {
		log.Error(err, "failed to parse clean")
		return ctrl.Result{}, fmt.Errorf("failed to parse clean: %v", err)
	}

	handleEnable := func() (ctrl.Result, error) {
		vmnw.Status.Phase = "Running"
		if err := r.Status().Update(ctx, vmnw); err != nil {
			log.Error(err, "Update resource status")
			return ctrl.Result{}, err
		}
		if r.cleanupTimer != nil {
			r.cleanupTimer.Reset(clean)
		} else {
			r.cleanupTimer = time.NewTicker(clean)
		}
		r.run = true
		go r.runWorker(r.ctx)
		// 初始化时间轮
		r.tw = timewheel.New(1*time.Second, 3600, func(data interface{}) {
			if dataMap, ok := data.(map[string]string); ok {
				if value, exists := dataMap["key"]; exists {
					r.workqueue.Add(value)
				}
			}
		})
		r.tw.Start()
		return ctrl.Result{}, nil
	}

	handleDisable := func() (ctrl.Result, error) {
		r.close()
		vmnw.Status.Phase = "Stopped"
		if err := r.Status().Update(ctx, vmnw); err != nil {
			log.Error(err, "Update resource status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	switch {
	case vmnw.Spec.Enable && !r.run:
		// 如果 Spec.Enable 为 true，并且之前未启动 worker
		return handleEnable()

	case !vmnw.Spec.Enable && r.run:
		// 如果 Spec.Enable 为 false，并且之前正在运行 worker
		return handleDisable()

	case vmnw.Spec.Enable:
		r.Log.Info(fmt.Sprintf("Update synchronization threshold to %s", r.interval))
		r.recorder.Event(&monitorv1.VirtualMachineNodeWatcher{}, corev1.EventTypeNormal, "Update Threshold", fmt.Sprintf("Update synchronization threshold to %s", r.interval))
		// 如果 Spec.Enable 为 true，并且之前已经启动 worker，则只重置清理定时器
		r.cleanupTimer.Reset(clean)
	}

	return ctrl.Result{}, nil
}

func (r *VirtualMachineNodeWatcherReconciler) close() {
	if r.tw != nil {
		r.tw.Stop()
	}
	if r.cleanupTimer != nil {
		r.cleanupTimer.Stop()
	}
	r.runWorkerStopCh <- struct{}{}
	r.run = false
}

func (r *VirtualMachineNodeWatcherReconciler) runWorker(ctx context.Context) {
	go func() {
		for {
			select {
			case <-r.cleanupTimer.C:
				r.cleanupResources()
			case <-r.runWorkerStopCh:
				r.Log.Info("Close channel signal received end runWorker")
				return
			case <-r.ctx.Done():
				r.Log.Info("Context canceled stop runWorker")
				return
			}
		}
	}()
	for r.processQueue(ctx) {
	}
}

// cleanupResources 清理状态为Succeeded、Cancel的VirtualMachineInstanceRescue资源
func (r *VirtualMachineNodeWatcherReconciler) cleanupResources() {
	defer r.recorder.Event(&monitorv1.VirtualMachineNodeWatcher{}, corev1.EventTypeNormal, "CleanedComplete", "Cleaned of VirtualMachineInstanceRescue complete")
	vimml, err := r.vmm.List(&metav1.ListOptions{})
	if err != nil {
		r.Log.Error(err, "Failed to obtain the list of VirtualMachineInstanceRescue resources")
		return
	}

	for _, vimm := range vimml.Items {
		if vimm.Status.Phase == monitorv1.MigrationSucceeded || vimm.Status.Phase == monitorv1.MigrationCancel {
			if err := r.vmm.Delete(vimm.Name, vimm.Namespace, &client.DeleteOptions{}); err != nil {
				r.Log.Error(err, "Failed to delete VirtualMachineInstanceRescue resource", "Name", vimm.Name, "Namespace", vimm.Namespace)
			}
		}
	}
}

// addMigration 向虚拟机迁移列表中添加新的虚拟机
func (r *VirtualMachineNodeWatcherReconciler) addMigration(name string, mvm *migration, node string) {

	// 检查虚拟机是否满足加入迁移队列的条件
	// 检查虚拟机是否正在进行迁移。
	// 如果虚拟机没有在进行迁移并且可以进行迁移（即满足迁移条件）否则返回
	ok1, err := r.vm.IsMigrating(context.Background(), mvm.VMI.Name, mvm.VMI.Namespace)
	if err != nil {
		r.Log.Error(err, "Get Virtual Machine Migration Status")
		return
	}

	if ok1 || !mvm.VMI.IsMigratable() {
		return
	}

	// 检测虚拟机实例是否已经存在迁移列表中
	if _, ok := r.vmm.IsEligible(name); !ok {
		return
	}

	// 查找运行虚拟机的Pod，如果不存在或者状态为PodPending，认为虚拟机迁移任务已成功，在等待kubevirt重新创建Pod。
	// 目前使用的是强制删除Pod来使kubevirt重新创建从而实现虚拟机迁移，暂时无法追踪重新创建的Pod是否是上次迁移任务删除后创建的
	pod, err := r.node.GetPodByVMI(context.Background(), &mvm.VMI)
	if err != nil {
		r.Log.Error(err, "Get pod information")
		return
	}
	if pod == nil {
		return
	}

	newVmim := &monitorv1.VirtualMachineInstanceRescue{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vmrescuer-",
			Namespace:    mvm.VMI.Namespace,
		},
		Spec: monitorv1.VirtualMachineInstanceRescueSpec{
			VMI:  mvm.VMI.Name,
			Node: node,
		},
	}
	if _, err := r.vmm.Create(newVmim, &client.CreateOptions{}); err != nil {
		r.Log.Error(err, "Create migration resources")
		return
	}

	newVmim.Status.Phase = monitorv1.MigrationQueuing
	if _, err := r.vmm.UpdateStatus(newVmim); err != nil {
		r.Log.Error(err, "Failed to update queue information")
	}

}

func (r *VirtualMachineNodeWatcherReconciler) processQueue(ctx context.Context) bool {
	obj, shutdown := r.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func, so we can defer c.workqueue.Done.
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
			r.Log.Error(err, fmt.Sprintf("VirtualMachineInstance %s migration failed and rejoined the migration queue", key))
			// Put the item back on the workqueue to handle any transient errors.
			//r.workqueue.AddRateLimited(key)
			r.workqueue.Forget(obj)
			return fmt.Errorf("error syncing %s: %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens.
		r.workqueue.Forget(obj)
		//r.Log.Info(fmt.Sprintf("Successfully synced resource %s ", key))
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (r *VirtualMachineNodeWatcherReconciler) syncHandler(ctx context.Context, key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	res, err := r.vmm.Get(name, namespace, &client.GetOptions{})
	if err != nil {
		r.Log.Error(err, "Failed to obtain VirtualMachineInstanceRescue resource")
		return err
	}

	if res.Status.Phase == monitorv1.MigrationCancel {
		r.Log.Info(fmt.Sprintf("VirtualMachineInstanceRescue %s Canceled", key))
		return nil
	}

	// 根据 Namespace 和 Name 获取虚拟机实例对象
	vmi, err := r.vm.Get(ctx, res.Namespace, res.Spec.VMI)
	if err != nil {
		return err
	}

	// 检查虚拟机是否正在迁移
	isMigrating, err := r.vm.IsMigrating(ctx, vmi.Name, vmi.Namespace)
	if err != nil {
		return err
	}

	// 如果虚拟机正在迁移，则不进行迁移操作，直接返回
	if isMigrating {
		r.Log.Info(fmt.Sprintf("VMI %s is Migrating", vmi.Name))
		res.Status.Phase = monitorv1.MigrationSucceeded
		_, err = r.vmm.UpdateStatus(res)
		if err != nil {
			r.Log.Error(err, "Failed to update VirtualMachineInstanceRescue status")
		}
		return nil
	}

	log := r.Log.WithValues("vmi", vmi.Name, "namespace", vmi.Namespace, "sourceNode", vmi.Status.NodeName)

	log.Info("Start VirtualMachineMigration")

	// 执行虚拟机实例迁移 使用删除pod来替代使用kubevirt客户端的迁移
	ok, err := r.node.Migrate(ctx, vmi)
	if err != nil {
		return err
	}
	if !ok {
		log.Info("VirtualMachine migration failed")
		return nil
	}

	// 使用kubevirt客户端执行虚拟机实例迁移
	//if err = r.vm.Migrate(context.Background(), vmi.Name, namespace); err != nil {
	//	return err
	//}

	res.Status.Phase = monitorv1.MigrationSucceeded
	_, err = r.vmm.UpdateStatus(res)
	if err != nil {
		log.Error(err, "Failed to update VirtualMachineInstanceRescue status")
	}

	log.Info("VirtualMachine migration successful")

	// 返回 nil 表示启动迁移成功
	return nil
}

// syncVMToMigrate 获取故障节点需要迁移的虚拟机
func (r *VirtualMachineNodeWatcherReconciler) syncVMToMigrate(ctx context.Context) error {
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
	// 获取不健康节点的虚拟机实例
	UnhealthyVMIS, err := func(ctx context.Context) ([]virtv1.VirtualMachineInstance, error) {
		runningNodes, err := runningVirtualMachineNodes(ctx)
		if err != nil {
			return nil, err
		}

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
	for _, vmi := range UnhealthyVMIS {
		//ok, err := r.vm.IsMigrating(ctx, vmi.Name, vmi.Namespace)
		//if err != nil {
		//	r.Log.Error(err, "Get Virtual Machine Migration Status")
		//	continue
		//}
		//if !ok && vmi.IsMigratable() {
		//	r.addMigration(vmi.Name, &migration{VMI: vmi}, vmi.Status.NodeName)
		//}
		r.addMigration(vmi.Name, &migration{VMI: vmi}, vmi.Status.NodeName)
	}

	return nil
}

// SetupWithManager 注册 Informer 监听节点的变化
func (r *VirtualMachineNodeWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1.VirtualMachineNodeWatcher{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, handler.Funcs{UpdateFunc: r.nodeUpdateHandler}).
		Watches(&source.Kind{Type: &monitorv1.VirtualMachineInstanceRescue{}}, handler.Funcs{
			CreateFunc: r.vmirCreateHandler,
			DeleteFunc: r.vmirDeleteHandler,
			UpdateFunc: r.vmirUpdateHandler,
		}).
		Complete(r)
}
