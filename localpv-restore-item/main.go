/*
Copyright 20202 The OpenEBS Authors.

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

package main

import (
	"github.com/pkg/errors"

	veleroplugin "github.com/heptio/velero/pkg/plugin/framework"
	localpvrst "github.com/openebs/velero-plugin/pkg/localpv"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	veleroplugin.NewServer().
		RegisterRestoreItemAction("openebs.io/localpv-plugin", newRestorePlugin).
		Serve()
}

func newRestorePlugin(logger logrus.FieldLogger) (interface{}, error) {
	conf, err := rest.InClusterConfig()
	if err != nil {
		logger.Errorf("Failed to get cluster config : %s", err.Error())
		return nil, errors.Wrapf(err, "error fetching cluster config")
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		logger.Errorf("Error creating clientset : %s", err.Error())
		return nil, errors.Wrapf(err, "error creating k8s client")
	}

	configMapClient := clientset.CoreV1().ConfigMaps("velero")
	nodeClient := clientset.CoreV1().Nodes()

	return &localpvrst.RestorePlugin{
		Log:             logger,
		ConfigMapClient: configMapClient,
		NodeClient:      nodeClient,
	}, nil
}
