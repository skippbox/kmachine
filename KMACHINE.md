Kmachine notes
--------------

Currently proto works on Exoscale driver, we need to port it to all drivers.
Also need to handle auth properly.

Assuming you set your workstation with all the GOLANG goodies

Build a new binary via docker:

    $ make build
    Successfully built 063343bdccf5
    Number of parallel builds: 1

    -->      darwin/386: github.com/docker/machine
    -->    darwin/amd64: github.com/docker/machine
    -->       linux/386: github.com/docker/machine
    -->     linux/amd64: github.com/docker/machine
    -->       linux/arm: github.com/docker/machine
    -->     windows/386: github.com/docker/machine
    -->   windows/amd64: github.com/docker/machine

Then create a machine:

$ ./kmachine_darwin-amd64 create -d exoscale toto
Querying exoscale for the requested parameters...
Generate an SSH keypair...
Spawn exoscale host...
Waiting for job to complete...
To see how to connect Docker to this machine, run: kmachine_darwin-amd64 env toto

Get its environment variables

    $ ./kmachine_darwin-amd64 env toto
    kubectl config set-cluster kmachine --server=https://185.19.29.217:6443 --insecure-skip-tls-verify=true
    kubectl config set-credentials kuser --token=abcdefghijkl
    kubectl config set-context kmachine --user=kuser --cluster=kmachine
    kubectl config use-context kmachine
    export DOCKER_TLS_VERIFY="1"
    export DOCKER_HOST="tcp://185.19.29.217:2376"
    export DOCKER_CERT_PATH="/Users/sebastiengoasguen/.docker/machine/machines/toto"
    export DOCKER_MACHINE_NAME="toto"
    # Run this command to configure your shell: 
    # eval "$(kmachine_darwin-amd64 env toto)"

Set the variables for the clients to work:

    $ eval "$(./kmachine_darwin-amd64 env toto)"

Assuming you have installed the right `kubectl` cli, it will work

    $ kubectl get nodes

And docker also works with the client talking to the remote Docker engine

    $ docker ps

