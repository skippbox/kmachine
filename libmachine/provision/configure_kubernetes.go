package provision

import (
    "fmt"

    "github.com/docker/machine/libmachine/auth"
    //"github.com/docker/machine/libmachine/cert"
    "github.com/docker/machine/libmachine/kubernetes"
    "github.com/docker/machine/libmachine/log"
)

func configureKubernetes(p Provisioner, k8sOptions kubernetes.KubernetesOptions, authOptions auth.AuthOptions) (error) {
    log.Info("Configuring kubernetes...")

    /* Generate and install certificates. Then kick off kubernetes */
    //certDir := authOptions.CertDir

    /* Kick off the kubernetes run */
    if _, err := p.SSHCommand(fmt.Sprintf("echo %q > /tmp/tokenfile.txt", k8sOptions.K8SToken)); err != nil {
        return err
    }

    if _, err := p.SSHCommand("sudo /bin/sh /etc/rc.d/k8s.sh"); err != nil {
        return err
    }

    return nil
}