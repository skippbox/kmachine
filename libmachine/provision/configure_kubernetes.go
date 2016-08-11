package provision

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"text/template"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/cert"
	"github.com/docker/machine/libmachine/kubernetes"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/provision/serviceaction"
)

func xferCert(p Provisioner, certPath string, targetPath string) error {
	certXferCmd := "printf '%%s' '%s' | sudo tee %s"

	certContents, err := ioutil.ReadFile(certPath)
	if err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s", targetPath)); err != nil {
		return err
	}

	/*
	 * TODO: Until we start dynamically generating the configuration file,
	 * these must have a known naming convention on the machine.
	 */
	_, certFile := path.Split(certPath)
	certFile = strings.Split(certFile, "_")[1]

	if _, err := p.SSHCommand(fmt.Sprintf(certXferCmd, string(certContents), path.Join(targetPath, certFile))); err != nil {
		return err
	}

	return nil
}

func fixPermissions(p Provisioner, certPath string, targetPath string) error {
	_, certFile := path.Split(certPath)

	/*
	 * TODO: Until we start dynamically generating the configuration file,
	 * these must have a known naming convention on the machine.
	 */
	certFile = strings.Split(certFile, "_")[1]

	_, err := p.SSHCommand(fmt.Sprintf("sudo chmod 0400 %s", path.Join(targetPath, certFile)))
	if err != nil {
		return err
	}

	return nil
}

func GenerateCertificates(p Provisioner, k8sOptions kubernetes.KubernetesOptions, authOptions auth.AuthOptions) error {
	/* Generate and install certificates. Then kick off kubernetes */
	driver := p.GetDriver()
	machine := driver.GetMachineName()
	bits := 2048 // Based on the initial configuration
	targetDir := k8sOptions.K8SCertPath
	ip, err := driver.GetIP()
	if err != nil {
		return fmt.Errorf("Error retrieving address: %s", err)
	}

	err = cert.GenerateCert(
		[]string{ip, "10.0.0.1", "localhost"},
		k8sOptions.K8SAPICert,
		k8sOptions.K8SAPIKey,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		kubernetes.GenOrg(machine, "api"),
		bits)

	if err != nil {
		return fmt.Errorf("Error generating API cert: %s", err)
	}

	err = cert.GenerateCert(
		[]string{""},
		k8sOptions.K8SAdminCert,
		k8sOptions.K8SAdminKey,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		kubernetes.GenOrg(machine, "admin"),
		bits)

	if err != nil {
		return fmt.Errorf("Error generating Admin cert: %s", err)
	}

	err = cert.GenerateCert(
		[]string{},
		k8sOptions.K8SProxyCert,
		k8sOptions.K8SProxyKey,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		kubernetes.GenOrg(machine, "proxy"),
		bits)

	if err != nil {
		return fmt.Errorf("Error generating proxy cert: %s", err)
	}

	/* Copy certs into place */
	log.Info("Copying certs to the remote system...")

	/* Kick off the kubernetes run */
	if _, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s", targetDir)); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%q,%s,%d' |sudo tee %s", k8sOptions.K8SToken, k8sOptions.K8SUser, 0, path.Join(targetDir, "tokenfile.txt"))); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAPIKey, targetDir+"/apiserver"); err != nil {
		return err
	}

	if err := fixPermissions(p, k8sOptions.K8SAPIKey, targetDir+"/apiserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAPICert, targetDir+"/apiserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SProxyCert, targetDir+"/proxyserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SProxyKey, targetDir+"/proxyserver"); err != nil {
		return err
	}

	if err := fixPermissions(p, k8sOptions.K8SProxyKey, targetDir+"/proxyserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAdminCert, targetDir+"/kubelet"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAdminKey, targetDir+"/kubelet"); err != nil {
		return err
	}

	if err := fixPermissions(p, k8sOptions.K8SAdminKey, targetDir+"/kubelet"); err != nil {
		return err
	}

	/* Copy the CA cert to a known location */
	caCertContents, err := ioutil.ReadFile(authOptions.CaCertPath)
	if err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s/ca.pem", caCertContents, targetDir)); err != nil {
		return err
	}

	return nil
}

func configureKubernetes(p Provisioner, k8sOptions *kubernetes.KubernetesOptions, authOptions auth.AuthOptions) error {
	log.Info("Configuring kubernetes...")

	sysdresult, err := CheckSystemD(p)
	if err != nil {
		return err
	}

	if !sysdresult {
		if _, err := p.SSHCommand("sudo /bin/sh /etc/init.d/kubelet stop"); err != nil {
			log.Info("Error while attempting to stop the kubelet: %s", err)
		}
	}

	if err := GenerateCertificates(p, *k8sOptions, authOptions); err != nil {
		return err
	}

	/* Generate and install certificates. Then kick off kubernetes */
	driver := p.GetDriver()
	machine := driver.GetMachineName()
	targetDir := k8sOptions.K8SCertPath

	/* Generate and copy new YAML files to the target */

	apiserverConfigFile, err := GenerateApiserverManifest(targetDir, k8sOptions.K8SVersion)
	if err != nil {
		return err
	}

	controllerManagerConfigFile, err := GenerateControllerManagerManifest(targetDir, k8sOptions.K8SVersion)
	if err != nil {
		return err
	}

	schedulerConfigFile, err := GenerateSchedulerManifest(k8sOptions.K8SVersion)
	if err != nil {
		return err
	}

	proxyConfigFile, err := GenerateProxyManifest(k8sOptions.K8SVersion)
	if err != nil {
		return err
	}

	kubeletConfig, err := GenerateKubeletConfig(machine, targetDir)
	if err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p /etc/kubernetes/policies")); err != nil {
		return err
	}

	policyFile, err := GeneratePolicyFile(k8sOptions.K8SUser)
	if err != nil {
		return err
	}

	/* TOOD: The target manifest directory should be a parameter throughout here */
	/* Ensure that the kubernetes configuration directory exists */
	if _, err := p.SSHCommand("sudo mkdir -p /etc/kubernetes/manifests"); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", kubeletConfig, "/etc/kubernetes/kubelet.kubeconfig")); err != nil {
		return err
	}

	if _, err := p.SSHCommand("sudo rm -f /etc/kubernetes/manifests/kubernetes.yaml"); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", GetEtcdManifest(), "/etc/kubernetes/manifests/etcd.yaml")); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", apiserverConfigFile, "/etc/kubernetes/manifests/apiserver.yaml")); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", controllerManagerConfigFile, "/etc/kubernetes/manifests/controller-manager.yaml")); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", schedulerConfigFile, "/etc/kubernetes/manifests/scheduler.yaml")); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", proxyConfigFile, "/etc/kubernetes/manifests/proxy.yaml")); err != nil {
		return err
	}

	/* Generate the policy file */
	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", policyFile, "/etc/kubernetes/policies/policy.jsonl")); err != nil {
		return err
	}

	/* Lastly, start the kubelet */
	if err := p.Service("kubelet", serviceaction.Start); err != nil {
		return err
	}

	return nil
}

func GeneratePolicyFile(name string) (string, error) {
	type ConfigDetails struct {
		Username string
	}
	var result bytes.Buffer

	details := ConfigDetails{name}
	policyTmpl := `{"user":"{{.Username}}"}
{"user":"scheduler", "readonly": true, "resource": "pods"}
{"user":"scheduler", "resource": "bindings"}
{"user":"proxy", "resource": "services"}
{"user":"proxy", "resource": "endpoints"}
{"user":"kubelet",  "resource": "pods"}
{"user":"kubelet",  "resource": "nodes"}
{"user":"kubelet",  "readonly": true, "resource": "services"}
{"user":"kubelet",  "readonly": true, "resource": "endpoints"}
{"user":"kubelet", "resource": "events"}
}`

	t, err := template.New("PolicyTmpl").Parse(policyTmpl)
	if err != nil {
		return "", err
	}

	err = t.Execute(&result, details)

	return result.String(), err
}

func GenerateKubeletConfig(name string, targetDir string) (string, error) {
	type ConfigDetails struct {
		ClusterName string
		CertDir     string
	}

	details := ConfigDetails{name, targetDir}
	var result bytes.Buffer

	kubeletConfigTmpl := `apiVersion: v1
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

	t, err := template.New("kubeletConfigTmpl").Parse(kubeletConfigTmpl)
	if err != nil {
		return "", err
	}

	err = t.Execute(&result, details)

	return result.String(), err
}

// GetEtcdManifest returns a manifest for an etcd pod
func GetEtcdManifest() string {
	return `apiVersion: v1
kind: Pod
metadata:
  name: kube-etcd
  namespace: kube-system
spec:
  hostNetwork: true
  volumes:
  - name: data
    hostPath:
      path: /var/lib/etcd
  containers:
  - name: kube-etcd
    image: b.gcr.io/kuar/etcd:2.1.1
    args:
    - --data-dir=/var/lib/etcd
    - --advertise-client-urls=http://127.0.0.1:2379
    - --listen-client-urls=http://127.0.0.1:2379
    - --listen-peer-urls=http://127.0.0.1:2380
    - --name=etcd
    volumeMounts:
    - name: data
      mountPath: /var/lib/etcd

`
}

// GenerateApiserverManifest generates a manifest for the apiserver
func GenerateApiserverManifest(targetDir string, version string) (string, error) {
	type ConfigDetails struct {
		CertDir string
		Version string
	}

	details := ConfigDetails{targetDir, version}

	configTmpl := `apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  hostNetwork: true
  volumes:
  - name: certs
    hostPath:
      path: {{.CertDir}}
  - name: policies
    hostPath:
      path: /etc/kubernetes/policies
  containers:
  - name: kube-apiserver
    image: gcr.io/google_containers/hyperkube-amd64:v{{.Version}}
    volumeMounts:
    - name: certs
      mountPath: {{.CertDir}}
      readOnly: true
    - name: policies
      mountPath: /etc/kubernetes/policies
      readOnly: true
    args:
    - /hyperkube
    - apiserver
    - --client-ca-file=/var/run/kubernetes/ca.pem
    - --token-auth-file={{.CertDir}}/tokenfile.txt
    - --allow-privileged=true
    - --service-cluster-ip-range=10.0.0.1/24
    - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,SecurityContextDeny,ResourceQuota
    - --insecure-bind-address=0.0.0.0
    - --insecure-port=8080
    - --secure-port=6443
    - --etcd-servers=http://127.0.0.1:2379
    - --tls-cert-file={{.CertDir}}/apiserver/cert.pem
    - --tls-private-key-file={{.CertDir}}/apiserver/key.pem
    - --v=2

`
	return generateConfig("apiserverConfig", configTmpl, details)
}

// GenerateControllerManagerManifest generates a manifest for the controller manager
func GenerateControllerManagerManifest(targetDir string, version string) (string, error) {
	type ConfigDetails struct {
		CertDir string
		Version string
	}

	details := ConfigDetails{targetDir, version}

	configTmpl := `apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  hostNetwork: true
  volumes:
  - name: certs
    hostPath:
      path: {{.CertDir}}
  containers:
  - name: kube-controller-manager
    image: gcr.io/google_containers/hyperkube-amd64:v{{.Version}}
    volumeMounts:
    - name: certs
      mountPath: {{.CertDir}}
      readOnly: true
    args:
    - /hyperkube
    - controller-manager
    - --master=http://127.0.0.1:8080
    - --service-account-private-key-file={{.CertDir}}/apiserver/key.pem
    - --root-ca-file=/var/run/kubernetes/ca.pem
    - --v=2

`
	return generateConfig("controllerManagerConfig", configTmpl, details)
}

// GenerateSchedulerManifest generates a manifest for the scheduler
func GenerateSchedulerManifest(version string) (string, error) {
	type ConfigDetails struct {
		Version string
	}

	details := ConfigDetails{version}

	configTmpl := `apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: gcr.io/google_containers/hyperkube-amd64:v{{.Version}}
    args:
    - /hyperkube
    - scheduler
    - --master=http://127.0.0.1:8080
    - --v=2

`
	return generateConfig("schedulerConfig", configTmpl, details)
}

// GenerateProxyManifest generates a manifest for the proxy
func GenerateProxyManifest(version string) (string, error) {
	type ConfigDetails struct {
		Version string
	}

	details := ConfigDetails{version}

	configTmpl := `apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: gcr.io/google_containers/hyperkube-amd64:v{{.Version}}
    securityContext:
      privileged: true
    args:
    - /hyperkube
    - proxy
    - --master=http://127.0.0.1:8080
    - --v=2

`
	return generateConfig("proxyConfig", configTmpl, details)
}

func generateConfig(tmplName string, tmpl string, details interface{}) (string, error) {
	var result bytes.Buffer
	t, err := template.New(tmplName).Parse(tmpl)
	if err != nil {
		return "", err
	}
	err = t.Execute(&result, details)
	return result.String(), err
}
