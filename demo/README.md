# demo

Three small apps to smoke-test a Kubernetes cluster. Each one has its own
namespace, Deployment, Service and Ingress, and exercises a different part of
the cluster.

| App      | Stack          | What it tests                                                       |
| -------- | -------------- | ------------------------------------------------------------------- |
| `pouet`  | nginx (static) | Ingress, Services, load balancing across 3 replicas                 |
| `kaboom` | Go (stdlib)    | Probes, restarts, OOMKilled, HPA (needs metrics-server), downward API |
| `miam`   | Node.js + Redis | Service DNS between tiers, StatefulSet, PVC (default StorageClass) |

## pouet

A big honk button. Every honk calls `/whoami` and the page tallies which pod
answered. Use "Rafale de 20" to see how requests spread across replicas.

Go through the Ingress to see the spread: `kubectl port-forward` always talks to
a single pod.

## kaboom

A demolition panel. Each button hits whichever pod the Service picks:

- **Chauffer le CPU**: spins one core for 30 s. Click a few times and watch
  `kubectl get hpa -n kaboom -w`.
- **Gaver de RAM**: +64 MiB per click. The limit is 256 MiB, so the 4th click
  gets the pod OOMKilled.
- **Porter pâle**: `/readyz` returns 503 for 30 s, the pod leaves the Service
  endpoints without restarting.
- **Tout faire péter**: the process exits with code 1, kubelet restarts it.

## miam

A dinner vote. Votes live in a Redis StatefulSet backed by a PVC, so every
`miam` pod shows the same scoreboard and votes survive restarts. Delete the
Redis pod to see the page report the outage, then recover with votes intact.

## Build and deploy

```sh
cd demo

# Local images for the current architecture
make build

# Multi-arch images (amd64 + arm64) pushed to Docker Hub as tutanka01/<app>
make push

make deploy     # kubectl apply -k .
make undeploy   # kubectl delete -k .
```

Bump `TAG` (`make push TAG=0.2.0`) together with `newTag` in
[`kustomization.yaml`](kustomization.yaml).

The Ingresses use the cluster's default IngressClass and the hosts
`pouet.demo.local`, `kaboom.demo.local` and `miam.demo.local`. Map them in
`/etc/hosts` or patch the hosts for your cluster. Without an Ingress controller:

```sh
kubectl port-forward -n miam svc/miam 8080:80
```
