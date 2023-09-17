# Using Netmux with Kubernetes

## Simple direct service
The example below tells netmux that a service called sample01 will be exposed
and connections on port 8080 will be redirected to the service pods also at 8080.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: sample
  annotations:
    nx: |-
      - name: sample01
spec:
  ports:
    - port: 8080
      name: sample
      protocol: TCP
      targetPort: 8080

  selector:
    app: sample
```

## Reverse service

When you want the cluster to connect to your machine a reverse connection
needs to be described as below.

The service in this case will always point to netmux itself. The annotations will
tell the host that connections from this endpoint will be redirected to itself on 8081

```yaml
apiVersion: v1
kind: Service
metadata:
  name: sample-rev
  namespace: netmux
  labels:
    app: netmux
  annotations:
    nx: |-
      - localport: "8081"
        name: sample-rev
        direction: C2L
        auto: false
        proto: tcp
        localaddr: 127.0.0.1
spec:
  ports:
    - port: 8081
  selector:
    app: netmux
```

## Example: kafka cluster

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kafka
  namespace: netmux
  annotations:
    nx: |-
      - name: kafka
      - name: kafka-0
        localAddr: kafka-0.kafka.netmux.svc.cluster.local
        containerAddr: kafka-0.kafka.netmux.svc.cluster.local
      - name: kafka-1
        localAddr: kafka-1.kafka.netmux.svc.cluster.local
        containerAddr: kafka-1.kafka.netmux.svc.cluster.local
      - name: kafka-2
        localAddr: kafka-2.kafka.netmux.svc.cluster.local
        containerAddr: kafka-2.kafka.netmux.svc.cluster.local
  labels:
    app: kafka-app
spec:
  ports:
    - name: '9092'
      port: 9092
      protocol: TCP
      targetPort: 9092
  selector:
    app: kafka-app
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kafka
  namespace: netmux
  labels:
    app: kafka-app
spec:
  serviceName: kafka
  replicas: 3
  selector:
    matchLabels:
      app: kafka-app
  template:
    metadata:
      labels:
        app: kafka-app
    spec:
      containers:
        - name: kafka-container
          image: doughgle/kafka-kraft
          ports:
            - containerPort: 9092
            - containerPort: 9093
          env:
            - name: REPLICAS
              value: '3'
            - name: SERVICE
              value: kafka
            - name: NAMESPACE
              value: netmux
            - name: SHARE_DIR
              value: /mnt/kafka
            - name: CLUSTER_ID
              value: oh-sxaDRTcyAr6pFRbXyzA
            - name: DEFAULT_REPLICATION_FACTOR
              value: '3'
            - name: DEFAULT_MIN_INSYNC_REPLICAS
              value: '2'
          volumeMounts:
            - name: data
              mountPath: /mnt/kafka
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes:
          - "ReadWriteOnce"
        resources:
          requests:
            storage: "1Gi"
```

## Example Postgres

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-userconfig
  labels:
    app: postgres
data:
  POSTGRES_DB: "postgres"
  POSTGRES_USER: "postgres"
  POSTGRES_PASSWORD: "postgres"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  selector:
    matchLabels:
      app: postgres
  replicas: 1
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          envFrom:
            - configMapRef:
                name: postgres-userconfig
          image: postgres:latest
          imagePullPolicy: "IfNotPresent"
          ports:
            - containerPort: 5432
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: postgresdb
      volumes:
        - name: postgresdb
          persistentVolumeClaim:
            claimName: postgres-pvc
---
kind: Service
apiVersion: v1
metadata:
  name: postgres
  annotations:
    nx: |-
      - name: postgres
spec:
  selector:
    app: postgres
  ports:
    - port: 5432
  type: ClusterIP
```