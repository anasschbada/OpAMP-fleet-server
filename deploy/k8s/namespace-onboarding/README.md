# Onboarding a namespace (optional, opt-in)

This directory is for **application teams**, not for whoever runs the
`opamp-system` platform namespace. It has nothing to do with letting your
collectors connect to the OpAMP server -- that already works with zero
Kubernetes permissions (see `../../docs/RBAC.md` section 1). This is only
for the optional, not-yet-implemented, namespace-scoped read-only
cross-check described in `docs/RBAC.md` section 3.

## What it grants

`role-and-binding.yaml`, applied in your own namespace, lets the
`opamp-server` ServiceAccount (which lives in a different namespace) run
`get`/`list`/`watch` on `pods` and `configmaps` **in your namespace only**.
It cannot:
- read Secrets,
- write/delete anything,
- see any other namespace,
- see cluster-scoped resources (Namespaces, Nodes, ...) -- a `Role` cannot
  grant that no matter what it lists (see `docs/RBAC.md` section 2).

## How to apply it

You need whatever access you already use to manage your own namespace
(typically `edit` or `admin` on it) -- no cluster-admin, no
`ClusterRole`/`ClusterRoleBinding` involved anywhere in this step.

1. Copy `role-and-binding.yaml`.
2. Replace the three placeholders (`<TEAM_NAMESPACE>`,
   `<OPAMP_SERVER_NAMESPACE>`, `<OPAMP_SERVER_SERVICEACCOUNT>`) with real
   values -- ask whoever runs the platform namespace for the latter two.
3. `kubectl apply -f role-and-binding.yaml`

## How to revoke it

```
kubectl delete rolebinding opamp-server-readonly -n <TEAM_NAMESPACE>
kubectl delete role opamp-server-readonly -n <TEAM_NAMESPACE>
```

Nothing on the server side needs to change -- deleting the RoleBinding is
enough, immediately.

## How to audit it

```
kubectl get role,rolebinding -n <TEAM_NAMESPACE> -l ""   # or just: get role/rolebinding opamp-server-readonly
kubectl describe rolebinding opamp-server-readonly -n <TEAM_NAMESPACE>
```

Every namespace that has onboarded is independently visible this way --
there is no central list to keep in sync, and no single object anywhere in
the cluster that reveals every onboarded namespace at once.
