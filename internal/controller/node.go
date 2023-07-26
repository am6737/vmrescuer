package controller

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	virt "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeWatcherInterface 定义节点相关的操作接口
type NodeWatcherInterface interface {
	IsNodeReady(node *corev1.Node) bool
	Recovered()
	GetPodByVMI(ctx context.Context, vmi *virt.VirtualMachineInstance) (*corev1.Pod, error)
	DeletePod(ctx context.Context, pod *corev1.Pod) error
	Migrate(ctx context.Context, vmi *virt.VirtualMachineInstance) (bool, error)
}

// NodeWatcher 自定义节点操作的实现
type NodeWatcher struct {
	client.Client
}

func (n *NodeWatcher) Migrate(ctx context.Context, vmi *virt.VirtualMachineInstance) (bool, error) {
	pod, err := n.GetPodByVMI(ctx, vmi)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return true, nil
	}
	if err := n.DeletePod(ctx, pod); err != nil {
		return false, err
	}
	return true, nil
}

func (n *NodeWatcher) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	// 强制删除pod实现迁移虚拟机的效果
	if err := n.Delete(ctx, pod, client.GracePeriodSeconds(0)); err != nil {
		return err
	}
	return nil
}

func NewNodeWatcher(client client.Client) *NodeWatcher {
	return &NodeWatcher{client}
}

func (n *NodeWatcher) IsNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (n *NodeWatcher) GetPodByVMI(ctx context.Context, vmi *virt.VirtualMachineInstance) (*corev1.Pod, error) {
	// 使用标签选择器来查找包含特定虚拟机实例 UID 的 Pod
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labels.Set{"kubevirt.io/created-by": string(vmi.UID)})
	if err := n.List(ctx, podList, client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, err
	}

	// 筛选掉处于创建中或运行中且 Ready 条件为 False 的 Pod
	var candidatePods []*corev1.Pod
	for _, pod := range podList.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "False" {
				// 创建一个新的 Pod 对象并将其指针添加到 candidatePods 切片
				candidatePod := pod.DeepCopy()
				//fmt.Println(fmt.Sprintf("找到了需要删除的pod %s", candidatePod.Name))
				candidatePods = append(candidatePods, candidatePod)
			}
		}
	}

	// 检查是否找到了匹配的 Pod
	if len(candidatePods) == 0 {
		return nil, nil
	}

	// 在这里可能需要进一步逻辑来选择正确的 Pod，比如根据 Pod 的状态、所在节点等
	// 这里只是一个示例，假设选择第一个候选 Pod
	return candidatePods[0], nil
}

func (n *NodeWatcher) Recovered() {

}
