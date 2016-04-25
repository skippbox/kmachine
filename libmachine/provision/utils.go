package provision

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/cert"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/provision/serviceaction"
)

type DockerOptions struct {
	EngineOptions     string
	EngineOptionsPath string
}

type k8sOptions struct {
	k8sOptions     string
	k8sOptionsPath string
	k8sKubeletCfg  string
	k8sKubeletPath string
	k8sPolicyCfg   string
}

func randToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func upgradeSystem(p Provisioner) error {
	// We need to make sure the image has been upgraded as some later
	// steps may fail
	if output, err := p.SSHCommand(fmt.Sprintf("sudo DEBIAN_FRONTEND=noninteractive apt-get -y -o Dpkg::Options::='--force-confdef' -o Dpkg::Options::='--force-confnew' dist-upgrade")); err != nil {
		return fmt.Errorf("error upgrading aufs image: %s\n", output)
	}

	fmt.Printf("Rebooting system...\n")
	if output, err := p.SSHCommand(fmt.Sprintf("sudo reboot")); err != nil {
		return fmt.Errorf("error rebooting aufs image: %s\n", output)
	}

	drivers.WaitForSSH(p.GetDriver())

	if output, err := p.SSHCommand(fmt.Sprintf("sudo apt-get update")); err != nil {
		return fmt.Errorf("error refreshing packages: %s\n", output)
	}

	return nil
}

func installDockerGeneric(p Provisioner, baseURL string) error {
	// install docker - until cloudinit we use ubuntu everywhere so we
	// just install it using the docker repos
	if output, err := p.SSHCommand(fmt.Sprintf("sudo apt-get -y install linux-image-extra-$(uname -r)")); err != nil {
		return fmt.Errorf("error installing aufs image: %s\n", output)
	}
	if output, err := p.SSHCommand(fmt.Sprintf("if ! type docker; then curl -sSL %s | sh -; fi", baseURL)); err != nil {
		return fmt.Errorf("error installing docker: %s\n", output)
	}

	return nil
}

func makeDockerOptionsDir(p Provisioner) error {
	dockerDir := p.GetDockerOptionsDir()
	if _, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s", dockerDir)); err != nil {
		return err
	}

	return nil
}

func setRemoteAuthOptions(p Provisioner) auth.AuthOptions {
	dockerDir := p.GetDockerOptionsDir()
	authOptions := p.GetAuthOptions()

	// due to windows clients, we cannot use filepath.Join as the paths
	// will be mucked on the linux hosts
	authOptions.CaCertRemotePath = path.Join(dockerDir, "ca.pem")
	authOptions.ServerCertRemotePath = path.Join(dockerDir, "server.pem")
	authOptions.ServerKeyRemotePath = path.Join(dockerDir, "server-key.pem")

	return authOptions
}

func ConfigureAuth(p Provisioner) error {
	var (
		err error
	)

	driver := p.GetDriver()
	machineName := driver.GetMachineName()
	authOptions := p.GetAuthOptions()
	org := mcnutils.GetUsername() + "." + machineName
	bits := 2048

	ip, err := driver.GetIP()
	if err != nil {
		return err
	}

	log.Info("Copying certs to the local machine directory...")

	if err := mcnutils.CopyFile(authOptions.CaCertPath, filepath.Join(authOptions.StorePath, "ca.pem")); err != nil {
		return fmt.Errorf("Copying ca.pem to machine dir failed: %s", err)
	}

	if err := mcnutils.CopyFile(authOptions.ClientCertPath, filepath.Join(authOptions.StorePath, "cert.pem")); err != nil {
		return fmt.Errorf("Copying cert.pem to machine dir failed: %s", err)
	}

	if err := mcnutils.CopyFile(authOptions.ClientKeyPath, filepath.Join(authOptions.StorePath, "key.pem")); err != nil {
		return fmt.Errorf("Copying key.pem to machine dir failed: %s", err)
	}

	log.Debugf("generating server cert: %s ca-key=%s private-key=%s org=%s",
		authOptions.ServerCertPath,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		org,
	)

	// TODO: Switch to passing just authOptions to this func
	// instead of all these individual fields
	err = cert.GenerateCert(
		[]string{ip, "localhost"},
		authOptions.ServerCertPath,
		authOptions.ServerKeyPath,
		authOptions.CaCertPath,
		authOptions.CaPrivateKeyPath,
		org,
		bits,
	)

	if err != nil {
		return fmt.Errorf("error generating server cert: %s", err)
	}

	if err := p.Service("docker", serviceaction.Stop); err != nil {
		return err
	}

	// upload certs and configure TLS auth
	caCert, err := ioutil.ReadFile(authOptions.CaCertPath)
	if err != nil {
		return err
	}

	serverCert, err := ioutil.ReadFile(authOptions.ServerCertPath)
	if err != nil {
		return err
	}
	serverKey, err := ioutil.ReadFile(authOptions.ServerKeyPath)
	if err != nil {
		return err
	}

	log.Info("Copying certs to the remote machine...")

	// printf will choke if we don't pass a format string because of the
	// dashes, so that's the reason for the '%%s'
	certTransferCmdFmt := "printf '%%s' '%s' | sudo tee %s"

	// These ones are for Jessie and Mike <3 <3 <3
	if _, err := p.SSHCommand(fmt.Sprintf(certTransferCmdFmt, string(caCert), authOptions.CaCertRemotePath)); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf(certTransferCmdFmt, string(serverCert), authOptions.ServerCertRemotePath)); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf(certTransferCmdFmt, string(serverKey), authOptions.ServerKeyRemotePath)); err != nil {
		return err
	}

	dockerUrl, err := driver.GetURL()
	if err != nil {
		return err
	}
	u, err := url.Parse(dockerUrl)
	if err != nil {
		return err
	}
	dockerPort := 2376
	parts := strings.Split(u.Host, ":")
	if len(parts) == 2 {
		dPort, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
		dockerPort = dPort
	}

	dkrcfg, err := p.GenerateDockerOptions(dockerPort)
	if err != nil {
		return err
	}

	log.Info("Setting Docker configuration on the remote daemon...")

	if _, err = p.SSHCommand(fmt.Sprintf("printf %%s \"%s\" | sudo tee %s", dkrcfg.EngineOptions, dkrcfg.EngineOptionsPath)); err != nil {
		return err
	}

	if err := p.Service("docker", serviceaction.Start); err != nil {
		return err
	}

	return waitForDocker(p, dockerPort)
}

func installk8sGeneric(p Provisioner) error {
	k8scfg, err := p.Generatek8sOptions()
	if err != nil {
		return err
	}

	if _, err = p.SSHCommand("sudo mkdir /etc/kubernetes"); err != nil {
		return err
	}

	if _, err = p.SSHCommand(fmt.Sprintf("printf '%s' | sudo tee %s", k8scfg.k8sOptions, k8scfg.k8sOptionsPath)); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s'| sudo tee %s", k8scfg.k8sKubeletCfg, "/etc/kubernetes/kubelet.kubeconfig")); err != nil {
		return err
	}

	log.Debug("Installing kubelet dependencies...")
	if _, err := p.SSHCommand("sudo apt-get -y install socat"); err != nil {
		return err
	}

	log.Debug("Installing nsenter...")
	if _, err := p.SSHCommand("sudo docker run --rm -v /usr/local/bin:/target jpetazzo/nsenter"); err != nil {
		return err
	}

	log.Debug("Installing kubelet...")
	if _, err := p.SSHCommand(fmt.Sprintf("sudo curl -fL -o %s %s && sudo chmod +x %s",
		k8scfg.k8sKubeletPath,
		"https://storage.googleapis.com/kubernetes-release/release/v1.2.0/bin/linux/amd64/kubelet",
		k8scfg.k8sKubeletPath)); err != nil {
		return err
	}

	results, err := CheckSystemD(p)
	if err != nil {
		return err
	}

	if results {
		/* insert systemd service */
		results := `[Unit]
Description=Kubernetes Kubelet
Documentation=https://github.com/kubernetes/kubernetes

[Service]
ExecStart=/usr/local/bin/kubelet \
--api-servers=http://127.0.0.1:8080 \
--allow-privileged=true \
--config=/usr/local/etc/kubernetes/manifests \
--v=2
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target`
		if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee /lib/systemd/system/kubelet.service\n", results)); err != nil {
			return err
		}

		if err := p.Service("kubelet", serviceaction.Enable); err != nil {
			return err
		}
	} else {
		/* insert sysV service */
		results := `#!/bin/sh

run_start() {
	echo "Starting kubelet..."
		/usr/local/bin/kubelet --api-servers=http://127.0.0.1:8080 --cluster-dns=10.0.0.10 --cluster-domain=cluster.local --allow-privileged=true --config=/etc/kubernetes/manifests --kubeconfig=/etc/kubernetes/kubelet.kubeconfig --v=2 > /var/log/kubelet.log 2>&1 &
}

run_stop() {
	echo "Stopping kubelet..."
	P="x$(pidof kubelet)"

	if [ $P != "x" ]; then
		kill -9 $(pidof kubelet)
		for i in ` + "`docker ps|grep k8s|awk '{print $1}'`" + `; do
          docker kill -s 9 $i
        done
	fi
}

run_restart() {
	if pidof kubelet > /dev/null; then
		run_stop && run_start
	else
		run_start
	fi
}


case $1 in
	start) run_start;;
	stop) run_stop;;
	restart) run_restart;;
	*) echo "Usage $0 {start|stop|restart}"; exit 1
esac`
		if _, err := p.SSHCommand(fmt.Sprintf("printf '%%s' '%s' | sudo tee /etc/init.d/kubelet && sudo chmod +x /etc/init.d/kubelet\n", results)); err != nil {
			return err
		}
	}

	if _, err := p.SSHCommand("sudo mkdir -p /usr/local/etc/init.d && sudo ln -s /etc/init.d/kubelet /usr/local/etc/init.d/kubelet"); err != nil {
		return err
	}

	if _, err := p.SSHCommand("sudo ln -s /etc/init.d/kubelet /etc/rc3.d/S04kubelet"); err != nil {
		return err
	}

	if _, err := p.SSHCommand("sudo ln -s /etc/init.d/kubelet /etc/rc6.d/K01kubelet"); err != nil {
		return err
	}

	return nil
}

func matchNetstatOut(reDaemonListening, netstatOut string) bool {
	// TODO: I would really prefer this be a Scanner directly on
	// the STDOUT of the executed command than to do all the string
	// manipulation hokey-pokey.
	//
	// TODO: Unit test this matching.
	for _, line := range strings.Split(netstatOut, "\n") {
		match, err := regexp.MatchString(reDaemonListening, line)
		if err != nil {
			log.Warnf("Regex warning: %s", err)
		}
		if match && line != "" {
			return true
		}
	}

	return false
}

func checkDaemonUp(p Provisioner, dockerPort int) func() bool {
	reDaemonListening := fmt.Sprintf(":%d.*LISTEN", dockerPort)
	return func() bool {
		// HACK: Check netstat's output to see if anyone's listening on the Docker API port.
		netstatOut, err := p.SSHCommand("netstat -a")
		if err != nil {
			log.Warnf("Error running SSH command: %s", err)
			return false
		}

		return matchNetstatOut(reDaemonListening, netstatOut)
	}
}

func waitForDocker(p Provisioner, dockerPort int) error {
	if err := mcnutils.WaitForSpecific(checkDaemonUp(p, dockerPort), 5, 3*time.Second); err != nil {
		return NewErrDaemonAvailable(err)
	}

	return nil
}

func CheckSystemD(p Provisioner) (bool, error) {

	results, err := p.SSHCommand("readlink /sbin/init")
	if err != nil {
		/* If we error out then /sbin/init more than likely points to init */
		return false, nil
	}

	if strings.HasSuffix(results, "systemd") {
		return true, nil
	}

	return false, nil
}
