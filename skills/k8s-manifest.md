# kubernetes manifest patterns

## deployment
- always set resource requests/limits
- use liveness and readiness probes
- run as non-root
- use deployment + service pair

## example
```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{name}}
  namespace: kora-apps
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{name}}
  template:
    metadata:
      labels:
        app: {{name}}
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
        - name: {{name}}
          image: {{image}}
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "64Mi"
              cpu: "50m"
            limits:
              memory: "128Mi"
              cpu: "200m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 2
---
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: {{name}}
  namespace: kora-apps
spec:
  selector:
    app: {{name}}
  ports:
    - port: 80
      targetPort: 8080
```
