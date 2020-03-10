/*
Copyright 2020 The OpenEBS Authors.

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

package localpv

import (
	"fmt"

	"github.com/heptio/velero/pkg/plugin/framework"
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// RestorePlugin is a restore item action plugin for Velero
type RestorePlugin struct {
	Log             logrus.FieldLogger
	ConfigMapClient corev1client.ConfigMapInterface
	NodeClient      corev1client.NodeInterface
}

// AppliesTo returns information about which resources this action should be invoked for.
// A RestoreItemAction's Execute function will only be invoked on items that match the returned
// selector. A zero-valued ResourceSelector matches all resources.g
func (p *RestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

// Execute allows the RestorePlugin to perform arbitrary logic with the item being restored,
// in this case, setting a custom annotation on the item being restored.
func (p *RestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	p.Log.Info("Executing ChangePVCNodeAction")
	defer p.Log.Info("Done executing ChangePVCNodeAction")

	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{}, err
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	// check for localpv provisioner
	v, ok := annotations["volume.beta.kubernetes.io/storage-provisioner"]
	if !ok {
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	//TODO this check can be removed to use for other provisioner
	if v != "openebs.io/local" {
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	p.Log.Infof("Executing plugin for PVC %s", metadata.GetName())

	node, ok := annotations["volume.kubernetes.io/selected-node"]
	if !ok {
		p.Log.Debug("PVC doesn't have node selector")
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	// fetch node mapping from configMap
	newNode, err := getNewNodeFromConfigMap(p.ConfigMapClient, node)
	if err != nil {
		return nil, err
	}

	annotations["openebs.io/localpv-plugin"] = "1"

	if len(newNode) != 0 {
		// set node selector
		// We assume that node exist for node-mapping
		annotations["volume.kubernetes.io/selected-node"] = newNode
		metadata.SetAnnotations(annotations)
		p.Log.Infof("Updating selected-node for PVC %s to %s", metadata.GetName(), newNode)
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	// configMap doesn't have node-mapping
	// Let's check if node exists or not
	exists, err := isNodeExist(p.NodeClient, node)
	if err != nil {
		p.Log.Errorf("failed to check node existence: %s", err)
		return nil, errors.Wrapf(err, "error check node existence")
	}

	if !exists {
		p.Log.Infof("Resetting selected-node for PVC %s", metadata.GetName())
		delete(annotations, "volume.kubernetes.io/selected-node")
		metadata.SetAnnotations(annotations)
	}

	return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
}

func getNewNodeFromConfigMap(client corev1client.ConfigMapInterface, node string) (string, error) {
	// fetch node mapping from configMap
	config, err := getPluginConfig(framework.PluginKindRestoreItemAction, "velero.io/change-pvc-node", client)
	if err != nil {
		return "", err
	}

	if config == nil || len(config.Data) == 0 {
		// there is no node mapping defined for change-pvc-node
		// so we will return empty new node
		return "", nil
	}

	newNode, _ := config.Data[node]
	return newNode, nil
}

func getPluginConfig(kind framework.PluginKind, name string, client corev1client.ConfigMapInterface) (*corev1.ConfigMap, error) {
	opts := metav1.ListOptions{
		// velero.io/plugin-config: true
		// velero.io/restic: RestoreItemAction
		LabelSelector: fmt.Sprintf("velero.io/plugin-config,%s=%s", name, kind),
	}

	list, err := client.List(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(list.Items) == 0 {
		return nil, nil
	}

	if len(list.Items) > 1 {
		var items []string
		for _, item := range list.Items {
			items = append(items, item.Name)
		}
		return nil, errors.Errorf("found more than one ConfigMap matching label selector %q: %v", opts.LabelSelector, items)
	}

	return &list.Items[0], nil
}

func isNodeExist(nodeClient corev1client.NodeInterface, name string) (bool, error) {
	_, err := nodeClient.Get(name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
