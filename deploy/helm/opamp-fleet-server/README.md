# opamp-fleet-server Helm chart

Installs the OpAMP server + REST API + fleet UI. See the repository root
`README.md`, `docs/RBAC.md`, and `docs/SECURITY.md` for the full
architecture and security model — this file only covers chart usage.

## Quick start

Generate two **separate** token files first (never reuse one for both —
see `values.yaml`'s `auth` section comment for why):

```bash
openssl rand -base64 32 > agent-token.txt
openssl rand -base64 32 > api-token.txt
```

```bash
helm install my-fleet ./deploy/helm/opamp-fleet-server \
  --namespace opamp-system --create-namespace \
  --set server.image.repository=YOUR_REGISTRY/opamp-fleet-server \
  --set ui.image.repository=YOUR_REGISTRY/opamp-fleet-ui \
  --set auth.agentTokens.tokens="{$(cat agent-token.txt)}" \
  --set auth.apiTokens.tokens="{$(cat api-token.txt)}"
```

The chart refuses to render (`helm template`/`install` fails outright) if
`auth.agentTokens` and `auth.apiTokens` resolve to the same Secret —
see `templates/validations.yaml`.

## Recommended for anything beyond a quick try-out

Passing tokens via `--set`/values puts them in `helm get values` and your
shell history in plaintext. Prefer creating the Secrets yourself (kubectl,
External Secrets Operator, Sealed Secrets, ...) and pointing the chart at
them instead:

```bash
kubectl create secret generic my-agent-tokens -n opamp-system --from-file=tokens.txt=agent-token.txt
kubectl create secret generic my-api-tokens   -n opamp-system --from-file=tokens.txt=api-token.txt

helm install my-fleet ./deploy/helm/opamp-fleet-server \
  --namespace opamp-system --create-namespace \
  --set server.image.repository=YOUR_REGISTRY/opamp-fleet-server \
  --set ui.image.repository=YOUR_REGISTRY/opamp-fleet-ui \
  --set auth.agentTokens.existingSecret=my-agent-tokens \
  --set auth.apiTokens.existingSecret=my-api-tokens
```

## What this chart never creates

No `Role`, `RoleBinding`, `ClusterRole`, or `ClusterRoleBinding` — see
`docs/RBAC.md` in the repository root. The per-namespace opt-in Role
pattern described there is intentionally not part of this chart: it's
meant to be applied by other teams in their own namespaces when (if) a
future feature needs it, independent of installing/upgrading this release.

## Key values

| Value | Default | Notes |
|---|---|---|
| `server.image.repository` | placeholder | must point at your (mirrored, if airgapped) registry |
| `persistence.enabled` | `true` | disable only for a throwaway install -- loses fleet history on restart |
| `auth.agentTokens` / `auth.apiTokens` | empty | must be set to something (`tokens` or `existingSecret`), and must differ |
| `tls.enabled` | `false` | plaintext by default, for TLS-terminating meshes/ingresses |
| `networkPolicy.enabled` | `true` | |
| `ui.enabled` | `true` | set `false` to deploy only the server |

See `values.yaml` for the complete, commented list.

## Verifying a render before installing

```bash
helm lint ./deploy/helm/opamp-fleet-server
helm template test ./deploy/helm/opamp-fleet-server \
  --set auth.agentTokens.tokens='{dev-agent-token}' \
  --set auth.apiTokens.tokens='{dev-api-token}'
```
