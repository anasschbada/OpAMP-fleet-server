# OpAMP Fleet Server

Serveur [OpAMP](https://github.com/open-telemetry/opamp-spec) de production
pour piloter une flotte de collecteurs **OpenTelemetry, toutes
distributions confondues** (upstream `opentelemetry-collector-contrib`,
distribution vendor, build maison — tout ce qui embarque l'extension
`opamp`), plus une UI web de gestion de flotte. Conçu pour un cluster
Kubernetes **sécurisé, airgapped, sans accès admin** : voir
[`docs/RBAC.md`](docs/RBAC.md) pour le détail, mais en résumé, **le serveur
ne définit ni n'a besoin d'aucun `ClusterRole`, et fonctionne par défaut
avec zéro droit sur l'API Kubernetes.**

## Architecture

```
Collecteurs OTel ──OpAMP/WebSocket:4320──► opamp-server ──REST:8080──► UI React
(agent + gateway)  (identité, santé,        (registre en SQLite,
 dans vos namespaces  config effective,       historique des configs
 applicatifs)         push de config)         poussées)
                   ──HTTP (optionnel):8888──►  (scraping métriques
                                                 auto-télémétrie, si
                                                 l'agent l'a activé)
```

- **`cmd/opamp-server`** : le serveur — canal OpAMP (agents) + API REST
  (UI), stockage SQLite (pur Go, sans cgo), scraping optionnel des
  métriques d'auto-télémétrie des collecteurs.
- **`cmd/fleet-ui-server`** : petit serveur de fichiers statiques Go qui
  sert le build de `ui/` — pas de nginx, pour garder le même profil de
  dépendances/CVE que le serveur principal.
- **`ui/`** : UI React + TypeScript (Vite), recréée à partir du handoff de
  design (`design_handoff_opamp_fleet_manager/` si présent dans votre
  session) mais reconnectée à une vraie API au lieu de données mockées.
- **`deploy/k8s/`** : manifestes Kubernetes bruts (pas de Helm/Kustomize
  requis), voir plus bas.
- **`docs/RBAC.md`** : le modèle de permissions (aucun `ClusterRole`, opt-in
  par namespace).
- **`docs/SECURITY.md`** : résultats des scans de sécurité (gosec, npm
  audit) et ce qu'il reste à faire tourner en CI (govulncheck, scan
  d'image).

## Comment les informations remontent

Aucun appel à l'API Kubernetes n'est nécessaire : chaque collecteur
s'auto-décrit au serveur via le protocole OpAMP (`AgentDescription`,
`Health`, `EffectiveConfig`, `RemoteConfigStatus`), qui inclut ses propres
attributs Kubernetes (namespace, pod, nœud) obtenus via les variables
d'environnement Downward API — jamais un appel API. Le push de
configuration se fait dans l'autre sens sur ce même canal WebSocket
(`ServerToAgent.remote_config`), jamais via une modification de
`ConfigMap`. Voir `docs/RBAC.md` section 1 et
`deploy/k8s/collector-examples/README.md` pour le détail.

Les métriques opérationnelles (CPU, mémoire, débit, taux de succès export)
sont un mécanisme séparé et strictement optionnel : si un collecteur
annonce son port d'auto-télémétrie Prometheus (`opamp.fleetserver.self_metrics_port`
dans sa config), le serveur va le scraper — mais uniquement sur l'IP réelle
de la connexion OpAMP déjà authentifiée de cet agent, jamais sur une adresse
arbitraire (protection anti-SSRF, voir `internal/opampserver/registry.go`).

## Prérequis

- **Go 1.24+** et **Node.js 22+** pour builder localement.
- **Docker** (ou équivalent) pour construire les images — voir `Dockerfile`
  et `ui/Dockerfile`. En cluster airgapped, les images de base doivent être
  mirrorées dans votre registre interne au préalable (voir les commentaires
  en tête de chaque Dockerfile).
- **Un cluster Kubernetes** où vous pouvez créer des ressources namespaced
  standard (Deployment, Service, ConfigMap, Secret, PVC, NetworkPolicy)
  dans votre propre namespace — **aucun droit cluster-admin requis**.
- **Une StorageClass** disponible pour la PVC du registre SQLite (2Gi par
  défaut, voir `deploy/k8s/platform/02-pvc.yaml`).
- **Un jeton bearer** que vous générez vous-même (`openssl rand -base64
  32`) pour authentifier agents et UI — pas de SSO/OIDC intégré, voir
  `docs/SECURITY.md`.

## Démarrage en local (développement)

```bash
# Serveur
export AUTH_TOKENS_FILE=/tmp/tokens.txt
echo "dev-token" > /tmp/tokens.txt
export DATA_DIR=/tmp/opamp-dev
mkdir -p $DATA_DIR
go run ./cmd/opamp-server

# UI (dans un autre terminal)
cd ui
npm install
npm run dev
```

Ouvrez l'UI (URL affichée par `npm run dev`), entrez `dev-token` comme
jeton d'accès.

## Déploiement Kubernetes

```bash
kubectl apply -f deploy/k8s/platform/01-serviceaccount.yaml
kubectl apply -f deploy/k8s/platform/02-pvc.yaml
kubectl apply -f deploy/k8s/platform/03-configmap.yaml
# Générez de vrais jetons avant d'appliquer -- voir le commentaire dans ce fichier :
kubectl apply -f deploy/k8s/platform/04-secret-auth-tokens.example.yaml
kubectl apply -f deploy/k8s/platform/06-deployment.yaml
kubectl apply -f deploy/k8s/platform/07-service.yaml
kubectl apply -f deploy/k8s/platform/08-networkpolicy.yaml
```

(`00-namespace.yaml` est optionnel — voir son commentaire si vous n'avez
pas le droit de créer des namespaces vous-même.)

Puis, pour chaque namespace applicatif dont vous voulez piloter les
collecteurs, adaptez les manifestes dans
`deploy/k8s/collector-examples/` (voir son README pour le détail, en
particulier pourquoi c'est RBAC-free).

## Tests / vérifications effectués

- `go build ./...`, `go vet ./...`, `gofmt -l .` : propre.
- `gosec ./...` : 0 finding (voir `docs/SECURITY.md`).
- `npm audit` (UI) : 0 vulnérabilité.
- Smoke test end-to-end du serveur (démarrage, `/healthz`, `/readyz`,
  authentification REST, rejet WebSocket OpAMP non authentifié).
- `npm run build` + `tsc -b` pour l'UI : propre.

## Simplifications assumées par rapport au prototype de design d'origine

Pour livrer un backend de production solide dans le temps imparti, l'UI
réimplémente les 4 vues et le flux de push de config, mais simplifie deux
aspects du prototype visuel d'origine : pas d'éditeur YAML type
CodeMirror/Monaco (un `<textarea>` suffisamment fonctionnel), et le
constructeur de pipeline n'offre pas encore l'édition en ligne du YAML par
composant ni l'ajout de composants personnalisés ("Autre…"). La logique de
génération de config (union des composants par signal) et le tableau de
composants sont eux génériques (n'importe quelle distribution
OTel Collector), pas câblés en dur sur un vendor particulier.
