# redis-cluster built-in template

Redis Cluster chart used when the Redis service drawer is configured with `architecture=cluster`.

The user-facing service type remains `redis`; this chart is selected by PAAP at deploy time so the canvas keeps a single Redis card while using a real Redis Cluster Helm chart.
