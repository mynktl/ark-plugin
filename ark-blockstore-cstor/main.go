package main

import (
	veleroplugin "github.com/heptio/velero/pkg/plugin/framework"
	"github.com/sirupsen/logrus"
)

func main() {
	veleroplugin.NewServer().
		RegisterVolumeSnapshotter("velero.io/cstor-blockstore", openebsSnapPlugin).
		Serve()
}

func openebsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &BlockStore{Log: logger}, nil
}
