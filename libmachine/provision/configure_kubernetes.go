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
)

func xferCert(p Provisioner, certPath string, targetPath string) error {
	certXferCmd := "printf '%%s' '%s' | sudo tee %s"

	certContents, err := ioutil.ReadFile(certPath)
	if err != nil {
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
	if  err != nil {
		return err
	}

	return nil
}

func configureKubernetes(p Provisioner, k8sOptions *kubernetes.KubernetesOptions, authOptions auth.AuthOptions) (error) {
    log.Info("Configuring kubernetes...")

    /* CAB: Test theory that we can force an update by pushing a new manifest */
    if _, err := p.SSHCommand("sudo /bin/sh /usr/local/etc/init.d/kubelet stop"); err != nil {
        log.Info("Errored while attempting to stop the kubelet: %s", err)
    }

    /* Generate and install certificates. Then kick off kubernetes */
	driver := p.GetDriver()
	machine := driver.GetMachineName()
	bits := 2048	// Based on the initial configuration
	ip, err := driver.GetIP()
	if err != nil {
		return fmt.Errorf("Error retrieving address: %s", err)
	}

	err = cert.GenerateCert(
		[]string{ip, "localhost"},
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

	/* CAB: This should probably be an option */
	targetDir := k8sOptions.K8SCertPath

    /* Kick off the kubernetes run */
    if _, err := p.SSHCommand(fmt.Sprintf("printf '%q,%s,%d' |sudo tee %s", k8sOptions.K8SToken, "kuser",0,path.Join(targetDir, "tokenfile.txt"))); err != nil {
        return err
    }

	if err := xferCert(p, k8sOptions.K8SAPIKey, targetDir + "/apiserver"); err != nil {
		return err
	}

	if err := fixPermissions(p, k8sOptions.K8SAPIKey, targetDir + "/apiserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAPICert, targetDir + "/apiserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SProxyCert, targetDir + "/proxyserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SProxyKey, targetDir + "/proxyserver"); err != nil {
		return err
	}

	if err := fixPermissions(p, k8sOptions.K8SProxyKey, targetDir + "/proxyserver"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAdminCert, targetDir + "/kubelet"); err != nil {
		return err
	}

	if err := xferCert(p, k8sOptions.K8SAdminKey, targetDir + "/kubelet"); err != nil {
		return err
	}

	if err := fixPermissions(p, k8sOptions.K8SAdminKey, targetDir + "/kubelet"); err != nil {
		return err
	}

	/* Copy the CA cert to a known location */
	if _, err := p.SSHCommand(fmt.Sprintf("sudo cp /home/docker/.docker/ca.pem %s/ca.pem", targetDir)); err != nil {
		return err
	}

    /* Generate and copy a new YAML file to the target */
    configFile, err := Generatek8sManifest(machine, targetDir)
    if err != nil {
        return err
    }

    /* TOOD: The target manifest directory should be a parameter throughout here */
    if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee %s", configFile, "/etc/kubernetes/manifests/kubernetes.yaml")); err != nil {
        return err
    }

	/* Lastly, start the kubelet */
    if _, err := p.SSHCommand("sudo /bin/sh /usr/local/etc/init.d/kubelet start"); err != nil {
        return err
    }

    return nil
}

func Generatek8sManifest(name string, targetDir string) (string, error) {
    type ConfigDetails struct {
        ClusterName   string
        CertDir       string
    }

    details := ConfigDetails{name, targetDir}
    var result bytes.Buffer

    k8sConfigTmpl := `apiVersion: v1
kind: Pod
clusters:
  - cluster:
      certificate-authority: {{.CertDir}}/ca.pem
metadata:
  name: {{.ClusterName}}
spec:
  hostNetwork: true
  volumes:
    - name: "certs"
      hostPath:
        path: "{{.CertDir}}"
    - name: "policies"
      hostPath:
        path: "/etc/kubernetes/policies"
  containers:
    - name: "etcd"
      image: "b.gcr.io/kuar/etcd:2.1.1"
      args:
        - "--data-dir=/var/lib/etcd"
        - "--advertise-client-urls=http://127.0.0.1:2379"
        - "--listen-client-urls=http://127.0.0.1:2379"
        - "--listen-peer-urls=http://127.0.0.1:2380"
        - "--name=etcd"
    - name: "controller-manager"
      image: "gcr.io/google_containers/hyperkube:v1.0.3"
      args:
        - "/hyperkube"
        - "controller-manager"
        - "--master=http://127.0.0.1:8080"
        - "--v=2"
    - name: "apiserver"
      image: "gcr.io/google_containers/hyperkube:v1.0.3"
      volumeMounts:
        - name: "certs"
          mountPath: "{{.CertDir}}"
          readOnly: true
        - name: "policies"
          mountPath: "/etc/kubernetes/policies"
          readOnly: true
      args:
        - "/hyperkube"
        - "apiserver"
        - "--authorization-mode=AlwaysAllow"
        - "--client-ca-file=/var/run/kubernetes/ca.pem"
        - "--token-auth-file={{.CertDir}}/tokenfile.txt"
        - "--allow-privileged=true"
        - "--service-cluster-ip-range=10.0.20.0/24"
        - "--insecure-bind-address=0.0.0.0"
        - "--insecure-port=8080"
        - "--secure-port=6443"
        - "--etcd-servers=http://127.0.0.1:2379"
        - "--v=2"
    - name: "proxy"
      image: "gcr.io/google_containers/hyperkube:v1.0.3"
      securityContext:
        privileged: true
      args:
        - "/hyperkube"
        - "proxy"
        - "--master=http://127.0.0.1:8080"
        - "--v=2"
    - name: "scheduler"
      image: "gcr.io/google_containers/hyperkube:v1.0.3"
      args:
        - "/hyperkube"
        - "scheduler"
        - "--master=http://127.0.0.1:8080"
        - "--v=2"

`
    t, err := template.New("k8sConfig").Parse(k8sConfigTmpl)
    if err != nil {
        return "", err
    }

   err = t.Execute(&result, details)

    return result.String(), err
}