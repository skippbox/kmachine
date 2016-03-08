kmachine tips for windows users
===============================

There is a little step to add for windows users. The automation should come soon.

Docker-toolbox
--------------

For now, there are some issues when you try to run docker commands from windows terminals.
Docker-Toolbox comes with a `Quick Start Terminal` and we recommand to use it to simplify compatibility.

- [Download and Install docker-toolbox](https://www.docker.com/products/docker-toolbox).
- Then launch Docker Quickstart Terminal. 

You can remove the docker-toolbox virtual machine start by making a copy and updating `C:\Program Files\Docker Toolbox\start.sh`:
```bash
#!/bin/bash

docker () {
  MSYS_NO_PATHCONV=1 docker.exe "$@"
}
export -f docker

exec "${BASH}" --login -i
```  

Download kubectl
----------------
Move to your kmachine folder and add kubectl:
```console
$ VERSION="v1.1.8"
$ curl -L -O https://storage.googleapis.com/kubernetes-release/release/v${VERSION}/bin/windows/amd64/kubectl.exe
```

Start a kmachine
----------------
```bash
# Create a kmachine
./kmachine.exe create dev

# Save the env to a file 
./kmachine env dev > env.sh
```

Update the env.sh to match windows requirement (`kubectl` to `kubectl.exe` and update the certificate path):
```bash
kubectl.exe config set-cluster dev --server=https://192.168.99.101:6443 --insecure-skip-tls-verify=false
kubectl.exe config set-cluster dev --server=https://192.168.99.101:6443 --certificate-authority=/C/Users/shinmox/.kube/machine/machines/dev/ca.pem
kubectl.exe config set-credentials dev --token=9FyJ9DxrT8nR9hD7TxZO6TAckSioc2ux
kubectl.exe config set-context dev --user=dev --cluster=dev
kubectl.exe config use-context dev
export DOCKER_TLS_VERIFY="1"
export DOCKER_HOST="tcp://192.168.99.101:2376"
export DOCKER_CERT_PATH="C:\Users\shinmox\.kube\machine\machines\dev"
export DOCKER_MACHINE_NAME="dev"
# Run this command to configure your shell: 
# eval "$(C:\Users\shinmox\Programmes\kmachine_windows-amd64\kmachine.exe env dev)"
```

Finally, import the env parameters:
```console
. ./env.sh
```

Test the connection:
```console
$ docker ps
CONTAINER ID        IMAGE                                       COMMAND                  CREATED             STATUS              PORTS               NAMES
5f570abd5ec7        gcr.io/google_containers/hyperkube:v1.1.2   "/hyperkube apiserver"   6 minutes ago       Up 6 minutes                            k8s_apiserver.bf35c3a9_dev-dev_default_2c72cbfce09c6ed693219b1b83969698_b93d4deb
fb0e98a01fba        gcr.io/google_containers/hyperkube:v1.1.2   "/hyperkube controlle"   6 minutes ago       Up 6 minutes                            k8s_controller-manager.4abe0475_dev-dev_default_2c72cbfce09c6ed693219b1b83969698_b1c4c7e3
a1f6a9e9f2be        b.gcr.io/kuar/etcd:2.1.1                    "/etcd --data-dir=/va"   6 minutes ago       Up 6 minutes                            k8s_etcd.304a15d4_dev-dev_default_2c72cbfce09c6ed693219b1b83969698_bb70e4a4
a7003cbaec23        gcr.io/google_containers/hyperkube:v1.1.2   "/hyperkube scheduler"   6 minutes ago       Up 6 minutes                            k8s_scheduler.a601fd4c_dev-dev_default_2c72cbfce09c6ed693219b1b83969698_e5dbb7fd
c11065d03766        gcr.io/google_containers/hyperkube:v1.1.2   "/hyperkube proxy --m"   6 minutes ago       Up 6 minutes                            k8s_proxy.ad3d3c62_dev-dev_default_2c72cbfce09c6ed693219b1b83969698_590470d1
b2c101d6ed05        gcr.io/google_containers/pause:2.0          "/pause"                 7 minutes ago       Up 7 minutes                            k8s_POD.6059dfa2_dev-dev_default_2c72cbfce09c6ed693219b1b83969698_23d6e3ff
```

Done, you have a kmachine on windows.
