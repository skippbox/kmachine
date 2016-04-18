package commands


import (
	"bytes"
	"fmt"
	"strings"

	"github.com/docker/machine/cli"
	"github.com/kubernetes/helm/pkg/kubectl"
)

func generateK8sURL(url string) string {
	mParts := strings.Split(url, "://")
	mParts = strings.Split(mParts[1], ":")

	k8sHost := fmt.Sprintf("https://%s:6443", mParts[0])

	return k8sHost
}

func cmdDeploy(c *cli.Context) error {
	if len(c.Args()) != 2 {
		return fmt.Errorf("Requires a machine and deployment file location")
	}

	host, err := getFirstArgHost(c)
	if err != nil {
		return err
	}

	t := c.Args().Get(1)

	/* type for now can be one of: dns|helm|dashboard */
	var txt string
	switch (t) {
	case "dns":
		txt = `
######################################################################
# Copyright 2015 The Kubernetes Authors All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
######################################################################
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: dns
    name: dns
  name: dns`
	case "helm":
		txt = `
######################################################################
# Copyright 2015 The Kubernetes Authors All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
######################################################################
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: helm
    name: helm
  name: helm`
	case "dashboard":
		txt = `
######################################################################
# Copyright 2015 The Kubernetes Authors All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
######################################################################
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    app: kube-system
    name: kube-system
  name: kube-system
---
apiVersion: v1
kind: ReplicationController
metadata:
  # Keep the name in sync with image version and
  # gce/coreos/kube-manifests/addons/dashboard counterparts
  name: kubernetes-dashboard-v1.0.1
  namespace: kube-system
  labels:
    k8s-app: kubernetes-dashboard
    version: v1.0.1
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: kubernetes-dashboard
  template:
    metadata:
      labels:
        k8s-app: kubernetes-dashboard
        version: v1.0.1
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: kubernetes-dashboard
        image: gcr.io/google_containers/kubernetes-dashboard-amd64:v1.0.1
        resources:
          # keep request = limit to keep this container in guaranteed class
          limits:
            cpu: 100m
            memory: 50Mi
          requests:
            cpu: 100m
            memory: 50Mi
        ports:
        - containerPort: 9090
        livenessProbe:
          httpGet:
            path: /
            port: 9090
          initialDelaySeconds: 30
          timeoutSeconds: 30
---
apiVersion: v1
kind: Service
metadata:
  name: kubernetes-dashboard
  namespace: kube-system
  labels:
    k8s-app: kubernetes-dashboard
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: kubernetes-dashboard
  ports:
  - port: 80
    targetPort: 9090
`
	default:
		return fmt.Errorf("Invalid file: %s", t)
	}

	buf := bytes.NewBufferString(txt)

	url, err := host.Driver.GetURL(); if err != nil {
		return err
	}

	fmt.Println("using host: " + generateK8sURL(url))
	runner := &kubectl.RealRunner{}

	runner.Create(buf.Bytes())

	return nil
}