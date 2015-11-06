package provision

import (
    "fmt"
	"io/ioutil"
	"path"
    "strings"

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

	/* Lastly, start the kubelet */
    if _, err := p.SSHCommand("sudo /bin/sh /usr/local/etc/init.d/kubelet start"); err != nil {
        return err
    }

    return nil
}
