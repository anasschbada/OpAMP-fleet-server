# opamp-fleet-server Helm chart

Installs the OpAMP server + REST API + fleet UI. See the repository root
`README.md`, `docs/RBAC.md`, and `docs/SECURITY.md` for the full
architecture and security model — this file only covers chart usage.

## Quick start

```bash
helm install my-fleet ./deploy/helm/opamp-fleet-server \
  --namespace opamp-system --create-namespace \
  --set server.image.repository=YOUR_REGISTRY/opamp-fleet-server \
  --set ui.image.repository=YOUR_REGISTRY/opamp-fleet-ui
```

That's it — you don't need to generate tokens yourself first. With
`auth.agentTokens`/`auth.apiTokens` left at their defaults (no
`existingSecret`, no `tokens`), the chart **generates one random 40-char
token per pool itself** and prints how to retrieve it in the post-install
notes:

```bash
kubectl get secret my-fleet-opamp-fleet-server-agent-tokens -n opamp-system \
  -o jsonpath='{.data.tokens\.txt}' | base64 -d
```

Generated tokens are **stable across `helm upgrade`** (reused via `lookup`
against the existing Secret, never rotated silently — see
`templates/secret-agent-tokens.yaml`'s comment), so upgrading never breaks
collectors or UI sessions already configured with the old value.

The chart refuses to render (`helm template`/`install` fails outright) if
`auth.agentTokens` and `auth.apiTokens` resolve to the same Secret —
see `templates/validations.yaml`.

## Bringing your own tokens

Either pass an explicit list (ends up in `helm get values` in plaintext —
fine for a quick try-out, not for anything else):

```bash
helm install my-fleet ./deploy/helm/opamp-fleet-server \
  --set auth.agentTokens.tokens="{$(openssl rand -base64 32)}" \
  --set auth.apiTokens.tokens="{$(openssl rand -base64 32)}" \
  ...
```

or point at Secrets you created yourself (kubectl, External Secrets
Operator, Sealed Secrets, ...) — the recommended path for anything beyond a
quick try-out:

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

## Optional: HTTP Basic Auth as an alternate UI login

Off by default. When enabled, the REST API/UI accept either an API token
(above) or a single HTTP Basic Auth username/password — additive, never a
replacement for the token pools:

```bash
helm install my-fleet ./deploy/helm/opamp-fleet-server \
  --set auth.basicAuth.enabled=true \
  ...
```

The username defaults to `admin` (not randomly generated — you need to
actually know it); the password follows the exact same
existingSecret/explicit-value/auto-generated precedence as the token pools,
and is printed in the post-install notes when auto-generated. Set
`auth.basicAuth.username`/`auth.basicAuth.password` explicitly, or
`auth.basicAuth.existingSecret` to bring your own Secret (`username` +
`password` keys).

## Extra manifests

`extraManifests` renders arbitrary additional objects as part of this
release — most commonly an Ingress/IngressRoute in front of the server/UI
Services this chart creates (none is templated here on purpose, see the
root README's networking section). Each entry is passed through `tpl`, so
it can reference chart values/helpers. See `values.yaml`'s `extraManifests`
comment for a full example.

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
| `auth.agentTokens` / `auth.apiTokens` | auto-generated | override with `tokens` or `existingSecret`; must resolve to different Secrets |
| `auth.basicAuth.enabled` | `false` | additive HTTP Basic Auth login for the API/UI |
| `tls.enabled` | `false` | plaintext by default, for TLS-terminating meshes/ingresses |
| `networkPolicy.enabled` | `true` | |
| `ui.enabled` | `true` | set `false` to deploy only the server |
| `extraManifests` | `[]` | arbitrary additional manifests (Ingress, PrometheusRule, ...) |

See `values.yaml` for the complete, commented list.

## Verifying a render before installing

```bash
helm lint ./deploy/helm/opamp-fleet-server
helm template test ./deploy/helm/opamp-fleet-server
```
