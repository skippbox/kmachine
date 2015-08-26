package provision

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"crypto/rand"

	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/provision/pkgaction"
	"github.com/docker/machine/log"
	"github.com/docker/machine/utils"
)

type DockerOptions struct {
	EngineOptions     string
	EngineOptionsPath string
}

type k8sOptions struct {
	k8sOptions        string
	k8sOptionsPath    string
}

func randToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
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

	machineName := p.GetDriver().GetMachineName()
	authOptions := p.GetAuthOptions()
	org := machineName
	bits := 2048

	ip, err := p.GetDriver().GetIP()
	if err != nil {
		return err
	}

	// copy certs to client dir for docker client
	machineDir := filepath.Join(utils.GetMachineDir(), machineName)

	if err := utils.CopyFile(authOptions.CaCertPath, filepath.Join(machineDir, "ca.pem")); err != nil {
		log.Fatalf("Error copying ca.pem to machine dir: %s", err)
	}

	if err := utils.CopyFile(authOptions.ClientCertPath, filepath.Join(machineDir, "cert.pem")); err != nil {
		log.Fatalf("Error copying cert.pem to machine dir: %s", err)
	}

	if err := utils.CopyFile(authOptions.ClientKeyPath, filepath.Join(machineDir, "key.pem")); err != nil {
		log.Fatalf("Error copying key.pem to machine dir: %s", err)
	}

	log.Debugf("generating server cert: %s ca-key=%s private-key=%s org=%s",
		authOptions.ServerCertPath,
		authOptions.CaCertPath,
		authOptions.PrivateKeyPath,
		org,
	)

	// TODO: Switch to passing just authOptions to this func
	// instead of all these individual fields
	err = utils.GenerateCert(
		[]string{ip},
		authOptions.ServerCertPath,
		authOptions.ServerKeyPath,
		authOptions.CaCertPath,
		authOptions.PrivateKeyPath,
		org,
		bits,
	)

	if err != nil {
		return fmt.Errorf("error generating server cert: %s", err)
	}

	if err := p.Service("docker", pkgaction.Stop); err != nil {
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

	dockerUrl, err := p.GetDriver().GetURL()
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

	if _, err = p.SSHCommand(fmt.Sprintf("printf \"%s\" | sudo tee %s", dkrcfg.EngineOptions, dkrcfg.EngineOptionsPath)); err != nil {
		return err
	}

	if err := p.Service("docker", pkgaction.Start); err != nil {
		return err
	}

	// TODO: Do not hardcode daemon port, ask the driver
	if err := utils.WaitForDocker(ip, dockerPort); err != nil {
		return err
	}

	return nil
}

func installk8sGeneric(p Provisioner) error {
	// install Kubernetes in a single node using Docker containers
	//if output, err := p.SSHCommand("docker pull kubernetes/etcd"); err != nil {
	//	return fmt.Errorf("error installing k8s: %s\n", output)
	//}

	k8scfg, err := p.Generatek8sOptions()
	if err != nil {
			return err
	}

	if _, err = p.SSHCommand(fmt.Sprintf("printf '%s' | sudo tee %s", k8scfg.k8sOptions, k8scfg.k8sOptionsPath)); err != nil {
		return err
	}

	if _, err := p.SSHCommand(fmt.Sprintf("printf \"%s\" | sudo tee %s", "abcdefghijkl,machine,1000", "/tmp/tokenfile.txt")); err != nil {
		return err
	}

	log.Debug("launching master")
	if _, err := p.SSHCommand(fmt.Sprintf("sudo docker run -d --net=host --restart=always --name master -v /var/run/docker.sock:/var/run/docker.sock -v /tmp/master.json:/etc/kubernetes/manifests/master.json -v /tmp/tokenfile.txt:/tmp/tokenfile.txt gcr.io/google_containers/hyperkube:v1.0.3 /hyperkube kubelet --api_servers=http://localhost:8080 --v=2 --address=0.0.0.0 --enable_server --hostname_override=127.0.0.1 --config=/etc/kubernetes/manifests")); err != nil {
		return fmt.Errorf("error installing master")

	}

	return nil
}
