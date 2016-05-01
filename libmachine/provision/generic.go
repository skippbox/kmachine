package provision

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/engine"
	"github.com/docker/machine/libmachine/kubernetes"
	"github.com/docker/machine/libmachine/swarm"
)

type GenericProvisioner struct {
	OsReleaseId            string
	DockerOptionsDir       string
	DaemonOptionsFile      string
	KubernetesManifestFile string
	KubernetesKubeletPath  string
	Packages               []string
	OsReleaseInfo          *OsRelease
	Driver                 drivers.Driver
	AuthOptions            auth.AuthOptions
	EngineOptions          engine.EngineOptions
	SwarmOptions           swarm.SwarmOptions
	KubernetesOptions      kubernetes.KubernetesOptions
}

func (provisioner *GenericProvisioner) Hostname() (string, error) {
	return provisioner.SSHCommand("hostname")
}

func (provisioner *GenericProvisioner) SetHostname(hostname string) error {
	if _, err := provisioner.SSHCommand(fmt.Sprintf(
		"sudo hostname %s && echo %q | sudo tee /etc/hostname",
		hostname,
		hostname,
	)); err != nil {
		return err
	}

	// ubuntu/debian use 127.0.1.1 for non "localhost" loopback hostnames: https://www.debian.org/doc/manuals/debian-reference/ch05.en.html#_the_hostname_resolution
	if _, err := provisioner.SSHCommand(fmt.Sprintf(
		"if grep -xq 127.0.1.1.* /etc/hosts; then sudo sed -i 's/^127.0.1.1.*/127.0.1.1 %s/g' /etc/hosts; else echo '127.0.1.1 %s' | sudo tee -a /etc/hosts; fi",
		hostname,
		hostname,
	)); err != nil {
		return err
	}

	return nil
}

func (provisioner *GenericProvisioner) GetDockerOptionsDir() string {
	return provisioner.DockerOptionsDir
}

func (provisioner *GenericProvisioner) SSHCommand(args string) (string, error) {
	return drivers.RunSSHCommandFromDriver(provisioner.Driver, args)
}

func (provisioner *GenericProvisioner) CompatibleWithHost() bool {
	return provisioner.OsReleaseInfo.Id == provisioner.OsReleaseId
}

func (provisioner *GenericProvisioner) GetAuthOptions() auth.AuthOptions {
	return provisioner.AuthOptions
}

func (provisioner *GenericProvisioner) SetOsReleaseInfo(info *OsRelease) {
	provisioner.OsReleaseInfo = info
}

func (provisioner *GenericProvisioner) GetKubernetesOptions() kubernetes.KubernetesOptions {
	return provisioner.KubernetesOptions
}

func (provisioner *GenericProvisioner) Generatek8sOptions() (*k8sOptions, error) {
	type ConfigDetails struct {
		ClusterName string
		CertDir     string
                Version     string
	}

	var (
		k8sCfg        bytes.Buffer
		k8sKubeletCfg bytes.Buffer
	)

	configParams := ConfigDetails{
		provisioner.Driver.GetMachineName(),
		provisioner.KubernetesOptions.K8SCertPath,
                provisioner.KubernetesOptions.K8SVersion,
	}

	k8sKubeletConfigTmpl := `apiVersion: v1
kind: Config
clusters:
  - cluster:
      certificate-authority: {{.CertDir}}/ca.pem
      server: https://127.0.0.1:6443
    name: {{.ClusterName}}
contexts:
  - context:
      cluster: {{.ClusterName}}
      user: kubelet
    name: {{.ClusterName}}
users:
  - name: kubelet
    user:
      client-certificate: {{.CertDir}}/kubelet/cert.pem
      client-key: {{.CertDir}}/kubelet/key.pem`

	k8sConfigTmpl := `
{
"apiVersion": "v1",
"kind": "Pod",
"metadata": {"name":"{{.ClusterName}}"},
"spec":{
  "hostNetwork": true,
  "containers":[
    {
      "name": "etcd",
      "image": "b.gcr.io/kuar/etcd:2.1.1",
      "args": [
              "--data-dir=/var/lib/etcd",
              "--advertise-client-urls=http://127.0.0.1:2379",
              "--listen-client-urls=http://127.0.0.1:2379",
              "--listen-peer-urls=http://127.0.0.1:2380",
              "--name=etcd"
        ],
      "volumeMounts": [
          {"name": "data",
          "mountPath": "/var/lib/etcd"
          ]
    },
    {
      "name": "controller-manager",
      "image": "gcr.io/google_containers/hyperkube-amd64:v{{.Version}}",
      "volumeMounts": [ 
          {"name": "certs",
          "mountPath": "{{.CertDir}}",
          "readOnly": true }
          ],
      "args": [
              "/hyperkube",
              "controller-manager",
              "--service-account-private-key-file={{.CertDir}}/apiserver/key.pem",
              "--root-ca-file=/var/run/kubernetes/ca.pem",
              "--master=http://127.0.0.1:8080",
              "--v=2"
        ]
    },
    {
      "name": "apiserver",
      "image": "gcr.io/google_containers/hyperkube-amd64:v{{.Version}}",
      "volumeMounts": [ 
          {"name": "certs",
          "mountPath": "{{.CertDir}}",
          "readOnly": true },
          {"name": "policies",
          "mountPath": "/etc/kubernetes/policies",
          "readOnly": true }
          ],
      "args": [
              "/hyperkube",
              "apiserver",
              "--token-auth-file={{.CertDir}}/tokenfile.txt",
              "--client-ca-file=/var/run/kubernetes/ca.pem",
              "--allow-privileged=true",
              "--service-cluster-ip-range=10.0.0.1/24",
              "--admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,SecurityContextDeny,ResourceQuota",
              "--insecure-bind-address=0.0.0.0",
              "--insecure-port=8080",
              "--secure-port=6443",
              "--etcd-servers=http://127.0.0.1:2379",
              "--tls-cert-file={{.CertDir}}/apiserver/cert.pem",
              "--tls-private-key-file={{.CertDir}}/apiserver/key.pem",
              "--v=2"
        ]
    },
    {
      "name": "proxy",
      "image": "gcr.io/google_containers/hyperkube-amd64:v{{.Version}}",
      "securityContext": {
        "privileged": true
        },
      "args": [
              "/hyperkube",
              "proxy",
              "--master=http://127.0.0.1:8080",
              "--v=2"
        ]
    },
    {
      "name": "scheduler",
      "image": "gcr.io/google_containers/hyperkube-amd64:v{{.Version}}",
      "args": [
              "/hyperkube",
              "scheduler",
              "--master=http://127.0.0.1:8080",
              "--v=2"
        ]
    }
  ],
  "volumes":[
    { "name": "certs",
      "hostPath": {
        "path": "{{.CertDir}}"
      }
    }, { "name": "policies",
        "hostPath": {
            "path": "/etc/kubernetes/policies"
        }
    }, { "name": "data",
        "hostPath": {
            "path": "/var/lib/etcd"
        }
    }
  ]
 }
}
`
	t, err := template.New("k8sConfig").Parse(k8sConfigTmpl)
	if err != nil {
		return nil, err
	}

	kt, err := template.New("k8sKubeletConfig").Parse(k8sKubeletConfigTmpl)
	if err != nil {
		return nil, err
	}

	k8sPolicyCfg, err := GeneratePolicyFile(provisioner.KubernetesOptions.K8SUser)
	if err != nil {
		return nil, err
	}

	/*
		k8sContext := EngineConfigContext{
			DockerPort:    1234,
			AuthOptions:   provisioner.AuthOptions,
			EngineOptions: provisioner.EngineOptions,
		}
	*/

	//t.Execute(&k8sCfg, k8sContext)
	t.Execute(&k8sCfg, configParams)
	kt.Execute(&k8sKubeletCfg, configParams)

	return &k8sOptions{
		k8sOptions:     k8sCfg.String(),
		k8sOptionsPath: provisioner.KubernetesManifestFile,
		k8sKubeletPath: provisioner.KubernetesKubeletPath,
		k8sKubeletCfg:  k8sKubeletCfg.String(),
		k8sPolicyCfg:   k8sPolicyCfg,
	}, nil
}

func (provisioner *GenericProvisioner) GetOsReleaseInfo() (*OsRelease, error) {
	return provisioner.OsReleaseInfo, nil

}

func (provisioner *GenericProvisioner) GenerateDockerOptions(dockerPort int) (*DockerOptions, error) {
	var (
		engineCfg bytes.Buffer
	)

	driverNameLabel := fmt.Sprintf("provider=%s", provisioner.Driver.DriverName())
	provisioner.EngineOptions.Labels = append(provisioner.EngineOptions.Labels, driverNameLabel)

	engineConfigTmpl := `
DOCKER_OPTS='
-H tcp://0.0.0.0:{{.DockerPort}}
-H unix:///var/run/docker.sock
--storage-driver {{.EngineOptions.StorageDriver}}
--tlsverify
--tlscacert {{.AuthOptions.CaCertRemotePath}}
--tlscert {{.AuthOptions.ServerCertRemotePath}}
--tlskey {{.AuthOptions.ServerKeyRemotePath}}
{{ range .EngineOptions.Labels }}--label {{.}}
{{ end }}{{ range .EngineOptions.InsecureRegistry }}--insecure-registry {{.}}
{{ end }}{{ range .EngineOptions.RegistryMirror }}--registry-mirror {{.}}
{{ end }}{{ range .EngineOptions.ArbitraryFlags }}--{{.}}
{{ end }}
'
{{range .EngineOptions.Env}}export \"{{ printf "%q" . }}\"
{{end}}
`
	t, err := template.New("engineConfig").Parse(engineConfigTmpl)
	if err != nil {
		return nil, err
	}

	engineConfigContext := EngineConfigContext{
		DockerPort:    dockerPort,
		AuthOptions:   provisioner.AuthOptions,
		EngineOptions: provisioner.EngineOptions,
	}

	t.Execute(&engineCfg, engineConfigContext)

	return &DockerOptions{
		EngineOptions:     engineCfg.String(),
		EngineOptionsPath: provisioner.DaemonOptionsFile,
	}, nil
}

func (provisioner *GenericProvisioner) GetDriver() drivers.Driver {
	return provisioner.Driver
}
