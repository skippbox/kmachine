Kmachine notes
--------------

Currently proto works on Exoscale driver, we need to port it to all drivers.
Also need to handle auth properly.

Assuming you set your workstation with all the GOLANG goodies

Build a new binary via docker:
	$ export USE_CONTAINER=true
    $ make build
	docker build -t "docker-machine-build" .
	Sending build context to Docker daemon 18.07 MB
	Step 0 : FROM golang:1.5.1
	 ---> 7ce27d784f3a
	Step 1 : RUN go get  github.com/golang/lint/golint             github.com/mattn/goveralls             golang.org/x/tools/cover             github.com/tools/godep             github.com/aktau/github-release
	 ---> Running in 4ffff2d0821f
	 ---> 39073cf0593f
	Removing intermediate container 4ffff2d0821f
	Step 2 : WORKDIR /go/src/github.com/docker/machine
	 ---> Running in 11658eb43cdd
	 ---> 457719ce148f
	Removing intermediate container 11658eb43cdd
	Step 3 : ADD . /go/src/github.com/docker/machine
	 ---> a051c09beb4d
	Removing intermediate container 58a1322ed3ca
	Successfully built a051c09beb4d
	test -z '12972a87ea71        docker-machine-build   "make build"        3 minutes ago       Exited (0) 2 minutes ago                       docker-machine-build-container' || docker rm -f "docker-machine-build-container"
	docker-machine-build-container
	docker run --name "docker-machine-build-container" \
		    -e DEBUG \
		    -e STATIC \
		    -e VERBOSE \
		    -e BUILDTAGS \
		    -e PARALLEL \
		    -e COVERAGE_DIR \
		    -e TARGET_OS \
		    -e TARGET_ARCH \
		    -e PREFIX \
		    "docker-machine-build" \
		    make build
	WARNING: Your kernel does not support memory swappiness capabilities, memory swappiness discarded.
	go build -o /go/src/github.com/docker/machine/bin/docker-machine  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-exoscale  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-exoscale.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-digitalocean  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-digitalocean.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-vmwarevsphere  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-vmwarevsphere.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-none  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-none.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-rackspace  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-rackspace.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-azure  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-azure.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-amazonec2  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-amazonec2.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-virtualbox  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-virtualbox.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-vmwarefusion  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-vmwarefusion.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-softlayer  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-softlayer.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-hyperv  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-hyperv.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-google  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-google.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-openstack  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-openstack.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-vmwarevcloudair  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-vmwarevcloudair.go
	go build -o /go/src/github.com/docker/machine/bin/docker-machine-driver-generic  -tags "" -ldflags "-X `go list ./version`.GitCommit=`git rev-parse --short HEAD` -w -s"  cmd/machine-driver-generic.go
	test ! -d bin || rm -Rf bin
	test -z "build" || docker cp "docker-machine-build-container":/go/src/github.com/docker/machine/bin bin

Then create a machine:

$ PATH=$PATH:$PWD/bin bin/docker-machine create -d exoscale toto
Querying exoscale for the requested parameters...
Generate an SSH keypair...
Spawn exoscale host...
Waiting for job to complete...
To see how to connect Docker to this machine, run: docker-machine env toto

Get its environment variables

    $ PATH=$PATH:$PWD/bin docker-machine env toto
    kubectl config set-cluster kmachine --server=https://185.19.29.217:6443 --insecure-skip-tls-verify=true
    kubectl config set-credentials kuser --token=abcdefghijkl
    kubectl config set-context kmachine --user=kuser --cluster=kmachine
    kubectl config use-context kmachine
    export DOCKER_TLS_VERIFY="1"
    export DOCKER_HOST="tcp://185.19.29.217:2376"
    export DOCKER_CERT_PATH="/Users/sebastiengoasguen/.docker/machine/machines/toto"
    export DOCKER_MACHINE_NAME="toto"
    # Run this command to configure your shell: 
    # eval "$(bin/docker-machine env toto)"

Set the variables for the clients to work:

    $ eval "$(bin/docker-machine env toto)"

Assuming you have installed the right `kubectl` cli, it will work

    $ kubectl get nodes

And docker also works with the client talking to the remote Docker engine

    $ docker ps

