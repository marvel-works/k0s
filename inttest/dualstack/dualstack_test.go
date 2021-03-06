/*
Copyright 2020 Mirantis, Inc.

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

package dualstack

// Package implements basic smoke test for dualstack setup.
// Since we run tests under containers environment in the GHA we can't
// actually check proper network connectivity.
// Until wi migrate toward VM based test suites
// this test only checks that nodes in the cluster
// have proper values for spec.PodCIDRs

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/suite"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/inttest/common"
	k8s "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
)

type DualstackSuite struct {
	common.FootlooseSuite

	client *k8s.Clientset
}

func (ds *DualstackSuite) TestDualStackNodesHavePodCIDRs() {
	nl, err := ds.client.CoreV1().Nodes().List(context.Background(), v1meta.ListOptions{})
	ds.Require().NoError(err)
	for _, n := range nl.Items {
		ds.Require().Len(n.Spec.PodCIDRs, 2, "Each node must have ipv4 and ipv6 pod cidr")
	}

}

func (ds *DualstackSuite) getKubeConfig(node string) *restclient.Config {
	machine, err := ds.MachineForName(node)
	ds.Require().NoError(err)
	ssh, err := ds.SSH(node)
	ds.Require().NoError(err)
	kubeConf, err := ssh.ExecWithOutput("cat /var/lib/k0s/pki/admin.conf")
	ds.Require().NoError(err)
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	ds.Require().NoError(err)
	hostPort, err := machine.HostPort(6443)
	ds.Require().NoError(err)
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg
}

func (ds *DualstackSuite) SetupSuite() {
	ds.FootlooseSuite.SetupSuite()
	ds.prepareConfigWithDualStackEnabled()
	ds.Require().NoError(ds.InitMainController("/tmp/k0s.yaml", ""))
	ds.Require().NoError(ds.RunWorkers("/var/lib/k0s"))
	client, err := k8s.NewForConfig(ds.getKubeConfig("controller0"))
	ds.Require().NoError(err)
	err = ds.WaitForNodeReady("worker0", client)
	ds.Require().NoError(err)

	err = ds.WaitForNodeReady("worker1", client)
	ds.Require().NoError(err)

	ds.client = client

}

func TestDualStack(t *testing.T) {

	s := DualstackSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     2,
		},
		nil,
	}

	suite.Run(t, &s)

}

func (ds *DualstackSuite) prepareConfigWithDualStackEnabled() {
	ds.putFile("/tmp/k0s.yaml", k0sConfigWithAddon)
}

func (ds *DualstackSuite) putFile(path string, content string) {
	controllerNode := fmt.Sprintf("controller%d", 0)
	ssh, err := ds.SSH(controllerNode)
	ds.Require().NoError(err)
	defer ssh.Disconnect()
	_, err = ssh.ExecWithOutput(fmt.Sprintf("echo '%s' >%s", content, path))

	ds.Require().NoError(err)

}

const k0sConfigWithAddon = `
spec:
  network:
    calico:
      mode: "bird"
    dualStack:
      enabled: true
      IPv6podCIDR: "fd00::/108"
      IPv6serviceCIDR: "fd01::/108"
`
