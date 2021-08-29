package config

import (
	"fmt"
	kubekeyapiv1alpha1 "github.com/kubesphere/kubekey/apis/kubekey/v1alpha1"
	kubekeyclientset "github.com/kubesphere/kubekey/clients/clientset/versioned"
	kubekeycontroller "github.com/kubesphere/kubekey/controllers/kubekey"
	"github.com/kubesphere/kubekey/pkg/core/connector"
	"github.com/kubesphere/kubekey/pkg/core/logger"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

// todo: 原来inCluster的处理方式如何更加优雅，runtime中是否需要存operator有关的控制变量。普通创建集群，operator，webserver可以实现不同的runtime？
type Runtime struct {
	ObjName         string
	Cluster         *kubekeyapiv1alpha1.ClusterSpec
	Connector       connector.Connector
	Runner          *connector.Runner
	DownloadCommand func(path, url string) string
	AllNodes        []*kubekeyapiv1alpha1.HostCfg
	EtcdNodes       []*kubekeyapiv1alpha1.HostCfg
	MasterNodes     []*kubekeyapiv1alpha1.HostCfg
	WorkerNodes     []*kubekeyapiv1alpha1.HostCfg
	K8sNodes        []*kubekeyapiv1alpha1.HostCfg
	ClusterHosts    []string
	WorkDir         string
	Kubeconfig      string
	Conditions      []kubekeyapiv1alpha1.Condition
	ClientSet       *kubekeyclientset.Clientset
	Arg             Argument
}

type Argument struct {
	FilePath           string
	KubernetesVersion  string
	KsEnable           bool
	KsVersion          string
	Debug              bool
	SkipCheck          bool
	SkipPullImages     bool
	AddImagesRepo      bool
	DeployLocalStorage bool
	SourcesDir         string
	InCluster          bool
}

func NewRuntime(flag string, arg Argument) (*Runtime, error) {
	loader := NewLoader(flag, arg)
	cluster, err := loader.Load()
	if err != nil {
		return nil, err
	}
	clusterSpec := &cluster.Spec

	defaultCluster, hostGroups, err := clusterSpec.SetDefaultClusterSpec(arg.InCluster)
	if err != nil {
		return nil, err
	}

	var clientset *kubekeyclientset.Clientset
	if arg.InCluster {
		c, err := kubekeycontroller.NewKubekeyClient()
		if err != nil {
			return nil, err
		}
		clientset = c
	}

	r := &Runtime{
		ObjName:      cluster.Name,
		Cluster:      defaultCluster,
		Connector:    connector.NewDialer(),
		AllNodes:     hostGroupsPoint(hostGroups.All),
		EtcdNodes:    hostGroupsPoint(hostGroups.Etcd),
		MasterNodes:  hostGroupsPoint(hostGroups.Master),
		WorkerNodes:  hostGroupsPoint(hostGroups.Worker),
		K8sNodes:     hostGroupsPoint(hostGroups.K8s),
		ClusterHosts: generateHosts(hostGroups, defaultCluster),
		WorkDir:      generateWorkDir(),
		ClientSet:    clientset,
		Arg:          arg,
	}
	return r, nil
}

func hostGroupsPoint(hosts []kubekeyapiv1alpha1.HostCfg) []*kubekeyapiv1alpha1.HostCfg {
	arr := make([]*kubekeyapiv1alpha1.HostCfg, 0, len(hosts))
	for i := range hosts {
		arr = append(arr, &hosts[i])
	}
	return arr
}

// Copy is used to create a copy for Runtime.
func (r *Runtime) Copy() *Runtime {
	runtime := *r
	return &runtime
}

func generateHosts(hostGroups *kubekeyapiv1alpha1.HostGroups, cfg *kubekeyapiv1alpha1.ClusterSpec) []string {
	var lbHost string
	var hostsList []string

	if cfg.ControlPlaneEndpoint.Address != "" {
		lbHost = fmt.Sprintf("%s  %s", cfg.ControlPlaneEndpoint.Address, cfg.ControlPlaneEndpoint.Domain)
	} else {
		lbHost = fmt.Sprintf("%s  %s", hostGroups.Master[0].InternalAddress, cfg.ControlPlaneEndpoint.Domain)
	}

	for _, host := range cfg.Hosts {
		if host.Name != "" {
			hostsList = append(hostsList, fmt.Sprintf("%s  %s.%s %s", host.InternalAddress, host.Name, cfg.Kubernetes.ClusterName, host.Name))
		}
	}

	hostsList = append(hostsList, lbHost)
	return hostsList
}

func generateWorkDir() string {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		logger.Log.Fatal(errors.Wrap(err, "Failed to get current dir"))
	}
	return fmt.Sprintf("%s/%s", currentDir, kubekeyapiv1alpha1.DefaultPreDir)
}
