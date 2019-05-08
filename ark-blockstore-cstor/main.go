package main

import (
	veleroplugin "github.com/heptio/velero/pkg/plugin"
	"github.com/sirupsen/logrus"
)

func main() {
	veleroplugin.NewServer(veleroplugin.NewLogger()).
		RegisterBlockStore("cstor-blockstore", openebsSnapPlugin).
		Serve()
}

func openebsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &BlockStore{Log: logger}, nil
}
