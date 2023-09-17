# Installation Guide

There are multiple possibilities to apply netmux, this guide will explore each as they become available.

1. Kubernetes - Please check [guide.](using-kubernetes.md)
2. Docker (In progress)
3. Bare Metal (In progress)

The following components will be required anyhow, and have specific installation guides depending on your OS:

1. Netmux Daemon
2. Netmux Cli

In order to build them you can call (once you clone the repo) the following command:

```shell
make my-bins
```

Your binaries will be created under `zarf/dist`

Please add `nx` to a folder in your path and `nx-daemon` to a dedicated folder - this place will vary depending on your 
os, so please see specific guides below.

## Installing the Daemon:

### Macos

1. Place the binary in a proper folder - we suggest /usr/local/nx-daemon
2. Copy the plist file to /Library/LaunchDaemons
3. Run `sudo launchctl load -w /Library/LaunchDaemons/nx-daemon.plist`

# Uninstall

1. Run `sudo launchctl unload -w /Library/LaunchDaemons/nx-daemon.plist`
2. Remove the binary and the plist file

# Manual start
`sudo launchctl start -w /Library/LaunchDaemons/nx-daemon.plist`

# Manual stop
`sudo launchctl stop -w /Library/LaunchDaemons/nx-daemon.plist`

### nx-daemon.plist

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>EnvironmentVariables</key>
    <dict>
      <key>PATH</key>
      <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:</string>
    </dict>
    <key>UserName</key>
	<string>root</string>
	 <key>GroupName</key>
    <string>wheel</string>
    <key>Label</key>
    <string>nx-daemon</string>
    <key>Program</key>
    <string>/usr/local/nx/nx-daemon</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>LaunchOnlyOnce</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/nx-daemon.stdout</string>
    <key>StandardErrorPath</key>
    <string>/tmp/nx-daemon.stderr</string>
	<key>WorkingDirectory</key>
	<string>/usr/local/nx</string>
  </dict>
</plist>
```

### Linux

To be added

### Windows

To be added

## Running the Daemon

The Daemon will look for a file called `netmux.yaml` in its working directory it is running.

This file describes the endpoints you may access.

At the moment, every change in this file will require netmux to be restarted.

```yaml
endpoints:
  - name: local
    endpoint: netmux:50000
    kubernetes:
      config: /Users/psimao/.kube/config
      namespace: netmux
      endpoint: netmux
      port: 50000
      context: orbstack
```

## Installing on Kubernetes

### RBAC 
Netmux will require special accesses to monitor your namespace, so we 1st will need to address rbac:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: netmux
rules:
  - apiGroups: [ "" ]
    resources: [ "nodes", "services", "pods", "endpoints" ]
    verbs: [ "get", "list", "watch" ]
  - apiGroups: [ "extensions" ]
    resources: [ "deployments" ]
    verbs: [ "get", "list", "watch" ]

---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: netmux

---

apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: netmux
subjects:
  - kind: ServiceAccount
    name: netmux # name of your service account
    namespace: netmux # this is the namespace your service account is in
roleRef: # referring to your ClusterRole
  kind: Role
  name: netmux
  apiGroup: rbac.authorization.k8s.io

```

### Deployment

Once RBAC is set we can deploy it like this:

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: netmux
spec:
  replicas: 1
  selector:
    matchLabels:
      app: netmux
  template:
    metadata:
      labels:
        app: netmux
    spec:
      serviceAccountName: netmux

      containers:
        - name: netmux
          image: duxthemux/netmux:latest
          imagePullPolicy: IfNotPresent
          livenessProbe:
            httpGet:
              port: 8083
              path: /live
          readinessProbe:
            httpGet:
              port: 8083
              path: /ready
            initialDelaySeconds: 5
            periodSeconds: 5
          env:
            - name: LOGLEVEL
              value: debug
            - name: LOGSRC
              value: "false"
          ports:
            - containerPort: 50000
              protocol: TCP
              name: netmux-data
            - containerPort: 8082
              protocol: TCP
              name: prometheus
            - containerPort: 8083
              protocol: TCP
              name: probes

      imagePullSecrets:
        - name: reg
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: netmux
  namespace: netmux
  annotations:
    prometheus.io/scrape: 'true'
    prometheus.io.scheme: "http"
    prometheus.io/port: "8082"
    nx: |-
      - name: netmux-prom
        direction: L2C
        remotePort: "8082"
        localPort: "8082"
        localAddr: netmux-prom
spec:
  ports:
    - port: 50000
      name: netmux
      protocol: TCP
      targetPort: 50000
    - port: 8082
      name: prometheus
      protocol: TCP
      targetPort: 8082
    - port: 8083
      name: probes
      protocol: TCP
      targetPort: 8083
  selector:
    app: netmux
```