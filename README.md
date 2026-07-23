# OpAMP Fleet Server

Un serveur [OpAMP](https://github.com/open-telemetry/opamp-spec) + une UI web
pour piloter une flotte de collecteurs **OpenTelemetry** (n'importe quelle
distribution) depuis un seul endroit : qui est en vie, quelle config tourne
sur chaque collecteur, et pousser une nouvelle config à la volée.

Pensé pour tourner sur un cluster Kubernetes **sans droits admin** : le
serveur ne crée ni n'utilise aucun `ClusterRole`. Détail complet dans
[`docs/RBAC.md`](docs/RBAC.md).

## Comment ça marche

```
Collecteurs OTel ──OpAMP/WebSocket:4320──► opamp-server ──REST:8080──► UI React
(dans vos namespaces)                      (registre SQLite)
```

1. **Chaque collecteur s'auto-décrit** au serveur via le protocole OpAMP
   (identité, santé, config effective) — le serveur ne fait jamais d'appel à
   l'API Kubernetes pour savoir qui existe.
2. **Le serveur garde tout en base** (SQLite) : liste des agents, dernière
   config connue, historique des configs poussées.
3. **L'UI REST** lit ce registre et permet de pousser une nouvelle config à
   un agent ; le push repart sur le même canal WebSocket OpAMP, jamais via
   une modification de `ConfigMap`.
4. **Deux jetons séparés** protègent les deux canaux : un jeton "agent" pour
   le WebSocket OpAMP, un jeton "API" pour la REST/UI. Un pod collecteur
   compromis ne peut donc pas se servir de son jeton pour piloter les autres
   agents via l'API. Détail dans [`docs/SECURITY.md`](docs/SECURITY.md).
5. **Métriques optionnelles** : si un collecteur expose son port
   d'auto-télémétrie Prometheus, le serveur le scrape — uniquement sur l'IP
   déjà authentifiée de ce collecteur (pas d'adresse arbitraire, protection
   anti-SSRF).

## Structure du projet

```
.
├── cmd/
│   ├── opamp-server/       serveur principal (OpAMP + API REST)
│   └── fleet-ui-server/    petit serveur de fichiers statiques pour l'UI
├── internal/
│   ├── opampserver/        protocole OpAMP : handshake, push de config, registre en mémoire
│   ├── api/                API REST consommée par l'UI (routes, DTO, middlewares)
│   ├── auth/                lecture/validation des jetons agent et API
│   ├── ratelimit/          limite les tentatives d'auth échouées par IP
│   ├── store/              persistance (SQLite et mémoire, même interface)
│   ├── metrics/            scraping optionnel des métriques d'auto-télémétrie
│   └── config/             lecture de la config serveur (variables d'env)
├── ui/                     UI React + TypeScript (Vite)
├── deploy/
│   ├── k8s/                manifestes Kubernetes bruts (sans Helm)
│   └── helm/               chart Helm équivalent
├── docs/                   RBAC, sécurité, avis de prod-readiness
├── Dockerfile              image du serveur (opamp-server)
├── ui/Dockerfile           image de l'UI (fleet-ui-server)
└── .github/workflows/      CI : build/test/scan, images Docker, chart Helm
```

Détail des dossiers :

- **`cmd/opamp-server`** : point d'entrée du serveur — démarre le canal
  OpAMP (agents) et l'API REST (UI), ouvre le stockage SQLite.
- **`cmd/fleet-ui-server`** : sert le build statique de `ui/` en Go pur (pas
  de nginx), pour garder le même profil de dépendances/CVE que le serveur.
- **`internal/opampserver`** : implémentation du protocole OpAMP côté
  serveur — handshake, health, config effective, push de remote config,
  nettoyage des agents déconnectés (`sweeper.go`).
- **`internal/api`** : les routes REST utilisées par l'UI (liste des
  agents, catalogue de composants, push de config) et leurs middlewares
  (auth, logging).
- **`internal/auth`** : parsing et validation des deux fichiers de jetons
  (agent / API).
- **`internal/ratelimit`** : limiteur de débit par IP sur les échecs
  d'authentification (10/minute, au-delà 429).
- **`internal/store`** : interface de stockage du registre d'agents, avec
  deux implémentations (SQLite pour la prod, mémoire pour les tests).
- **`internal/metrics`** : parsing et scraping des métriques Prometheus
  d'auto-télémétrie exposées par les collecteurs (optionnel).
- **`internal/config`** : lecture de la configuration serveur depuis les
  variables d'environnement.
- **`ui/`** : UI React + TypeScript (Vite) — connectée à l'API REST
  ci-dessus, pas de données mockées.
- **`deploy/k8s`** : manifestes Kubernetes prêts à `kubectl apply`, sans
  Helm ni Kustomize.
- **`deploy/helm/opamp-fleet-server`** : chart Helm équivalent aux
  manifestes bruts (voir son propre README pour les valeurs).
- **`docs/RBAC.md`** : pourquoi aucun `ClusterRole` n'est nécessaire.
- **`docs/SECURITY.md`** : résultats des scans de sécurité (gosec,
  gitleaks, npm audit, pentest manuel).
- **`docs/PRODUCTION_READINESS.md`** : ce qui est prêt pour la prod et ce
  qui ne l'est pas encore.
- **`.github/workflows/`** : voir la section [CI/CD](#cicd) plus bas.

## Prérequis

- **Go 1.24+** et **Node.js 22+** pour builder localement.
- **Docker** pour construire les images (voir `Dockerfile` et
  `ui/Dockerfile`). En cluster airgapped, mirrorez d'abord les images de
  base dans votre registre interne (voir les commentaires en tête de
  chaque Dockerfile).
- **Un cluster Kubernetes** où vous pouvez créer des ressources namespaced
  standard (Deployment, Service, ConfigMap, Secret, PVC, NetworkPolicy) —
  **aucun droit cluster-admin requis**.
- **Une StorageClass** pour la PVC du registre SQLite (2Gi par défaut, voir
  `deploy/k8s/platform/02-pvc.yaml`).
- **Un jeton bearer** que vous générez vous-même (`openssl rand -base64
  32`) pour authentifier agents et UI — pas de SSO/OIDC intégré.

## Démarrage en local (développement)

```bash
# Serveur -- deux fichiers de jetons SÉPARÉS (voir "Sécurité" plus haut)
export AGENT_AUTH_TOKENS_FILE=/tmp/agent-tokens.txt
export API_AUTH_TOKENS_FILE=/tmp/api-tokens.txt
echo "dev-agent-token" > /tmp/agent-tokens.txt
echo "dev-api-token" > /tmp/api-tokens.txt
export DATA_DIR=/tmp/opamp-dev
mkdir -p $DATA_DIR
go run ./cmd/opamp-server

# UI (dans un autre terminal)
cd ui
npm install
npm run dev
```

Ouvrez l'UI (URL affichée par `npm run dev`), entrez `dev-api-token` comme
jeton d'accès.

## Déploiement Kubernetes

```bash
kubectl apply -f deploy/k8s/platform/01-serviceaccount.yaml
kubectl apply -f deploy/k8s/platform/02-pvc.yaml
kubectl apply -f deploy/k8s/platform/03-configmap.yaml
# Générez de vrais jetons avant d'appliquer -- voir le commentaire dans chaque fichier :
kubectl apply -f deploy/k8s/platform/04-secret-agent-tokens.example.yaml
kubectl apply -f deploy/k8s/platform/05-secret-api-tokens.example.yaml
kubectl apply -f deploy/k8s/platform/07-deployment.yaml
kubectl apply -f deploy/k8s/platform/08-service.yaml
kubectl apply -f deploy/k8s/platform/09-networkpolicy.yaml

# L'UI (jamais déployée sans ça)
kubectl apply -f deploy/k8s/platform/10-ui-serviceaccount.yaml
kubectl apply -f deploy/k8s/platform/11-ui-deployment.yaml
kubectl apply -f deploy/k8s/platform/12-ui-service.yaml
kubectl apply -f deploy/k8s/platform/13-ui-networkpolicy.yaml
```

(`00-namespace.yaml` est optionnel — voir son commentaire si vous n'avez pas
le droit de créer des namespaces vous-même.)

Puis, pour chaque namespace applicatif dont vous voulez piloter les
collecteurs, adaptez les manifestes dans `deploy/k8s/collector-examples/`
(voir son README) — utilisez un jeton **agent**, jamais celui de l'API.

### Avec Helm (alternative aux manifestes bruts)

```bash
helm install my-fleet ./deploy/helm/opamp-fleet-server \
  --namespace opamp-system --create-namespace \
  --set server.image.repository=YOUR_REGISTRY/opamp-fleet-server \
  --set ui.image.repository=YOUR_REGISTRY/opamp-fleet-ui \
  --set auth.agentTokens.existingSecret=my-agent-tokens \
  --set auth.apiTokens.existingSecret=my-api-tokens
```

Voir `deploy/helm/opamp-fleet-server/README.md` pour le détail des valeurs.
Le chart est aussi publié automatiquement en release GitHub, voir plus bas.

## CI/CD

Trois workflows dans `.github/workflows/` :

- **`ci.yml`** — sur chaque push/PR : build/vet/test/gosec/govulncheck
  (Go), build/typecheck/audit (UI), build des deux images Docker + scan
  Trivy, validation des manifestes/chart avec kubeconform et `helm lint`.
  Sur un tag `v*`, publie aussi les deux images sur **GHCR**
  (`ghcr.io/<owner>/opamp-fleet-server` et `opamp-fleet-ui`) et sur
  **Docker Hub** (`anasschb/images:opamp-fleet-server-<tag>` et
  `anasschb/images:opamp-fleet-ui-<tag>`, plus un tag `-latest`).
  Nécessite les secrets de repo `DOCKERHUB_USERNAME` et `DOCKERHUB_TOKEN`
  (un [access token](https://hub.docker.com/settings/security) Docker Hub,
  pas votre mot de passe).
- **`helm-release.yml`** — sur chaque push sur `main` qui touche
  `deploy/helm/**` : package le chart Helm et publie une **release
  GitHub** contenant le `.tgz` (tag `opamp-fleet-server-<version du
  Chart.yaml>`), et maintient un index Helm (`index.yaml`) sur la branche
  `gh-pages`. Pour publier une nouvelle version du chart : bump
  `version:` dans `deploy/helm/opamp-fleet-server/Chart.yaml` et pushez sur
  `main`.
- **`dependabot.yml`** — mises à jour automatiques des dépendances
  (Go, npm, GitHub Actions, Docker).

Pour publier une nouvelle version des images : créez un tag `vX.Y.Z` et
poussez-le (`git tag vX.Y.Z && git push origin vX.Y.Z`).

## Sécurité : deux jetons, pas un seul

Les collecteurs (canal OpAMP) et l'UI/les opérateurs (API REST) utilisent
**deux jeux de jetons séparés** (`AGENT_AUTH_TOKENS_FILE` /
`API_AUTH_TOKENS_FILE`). Le serveur refuse de démarrer si les deux
variables pointent vers le même fichier : un collecteur n'a besoin que
d'ouvrir une connexion OpAMP — s'il partageait aussi le jeton API, un seul
pod compromis pourrait pousser une configuration arbitraire à *tous* les
autres agents via l'API REST.

Les tentatives d'authentification échouées sont journalisées et limitées
en débit par IP (10 échecs/minute, au-delà : 429) — voir
`internal/ratelimit`.

## Tests / vérifications effectuées

- `go build ./...`, `go vet ./...`, `gofmt -l .` : propre.
- `go test ./... -race -cover` : toute la suite passe (auth, rate
  limiting, validation de config, stockage, parsing des métriques, logique
  OpAMP).
- `gosec ./...` : 0 finding. `gitleaks` sur tout l'historique : aucun
  secret trouvé. `npm audit` (UI) : 0 vulnérabilité.
- Pentest manuel (path traversal, injection SQL, injection CRLF, corps
  surdimensionnés, YAML malformé, CORS, rate limiting) : aucune faille
  trouvée à part un bug mineur déjà corrigé. Voir `docs/SECURITY.md`.
- `helm lint` + `helm template` + `kubeconform` sur le chart et les
  manifestes bruts : tous valides.
- `npm run build` + `tsc -b` pour l'UI : propre.

Voir `docs/PRODUCTION_READINESS.md` pour un avis honnête sur ce qui manque
encore avant une prod à enjeux critiques (haute disponibilité, SSO, tests
d'intégration bout-en-bout automatisés).

## Simplifications assumées par rapport au prototype de design d'origine

L'UI réimplémente les 4 vues et le flux de push de config, mais simplifie
deux aspects du prototype visuel d'origine : pas d'éditeur YAML type
CodeMirror/Monaco (un `<textarea>` suffisamment fonctionnel), et le
constructeur de pipeline n'offre pas encore l'édition en ligne du YAML par
composant ni l'ajout de composants personnalisés. La logique de génération
de config (union des composants par signal) et le tableau de composants
sont génériques (n'importe quelle distribution OTel Collector), pas
câblés en dur sur un vendor particulier.
