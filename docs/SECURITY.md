# Sécurité : ce qui a été vérifié, ce qui reste à vérifier en CI

## Analyse statique (gosec)

Exécuté sur tout le code Go de ce dépôt (`cmd/`, `internal/`) :

```
go install github.com/securego/gosec/v2/cmd/gosec@v2.22.5
gosec ./...
```

Résultat au moment de la livraison : **0 finding**, sur 22 fichiers /
~2400 lignes. Deux catégories de problèmes ont été corrigées pendant le
développement (pas juste ignorées) :

- **G115** (conversion `uint64`→`int64` non bornée) sur
  `Health.StartTimeUnixNano`, une valeur rapportée par l'agent — donc non
  fiable par construction. Corrigé en bornant à `math.MaxInt64` avant
  conversion (`internal/opampserver/handler.go`).
- **G104** (erreur ignorée) sur deux `Rollback()`/`Close()` dans les
  chemins d'erreur SQLite — ignorance délibérée (l'erreur d'origine prime),
  rendue explicite avec `_ = ...` plutôt que silencieuse.

Ré-exécutez `gosec ./...` à chaque changement — idéalement comme étape de
CI bloquante avant merge.

## Analyse de vulnérabilités des dépendances (govulncheck)

**Non exécuté avec succès dans cet environnement de développement** : la
politique réseau de ce sandbox bloque `vuln.go.dev` (403 côté proxy
sortant), qui héberge la base de vulnérabilités que `govulncheck`
interroge. Ce n'est pas une limitation du projet lui-même — c'est une
restriction de l'environnement où ce code a été écrit.

**À exécuter obligatoirement en CI avant toute release**, dans un
environnement qui a accès à `vuln.go.dev` (ou à un miroir interne de la
base si votre CI est elle-même isolée) :

```
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

`go mod verify` a été exécuté avec succès (toutes les sommes de contrôle
des modules correspondent à `go.sum`).

## Dépendances Go retenues (et pourquoi peu nombreuses)

Le nombre de dépendances tierces est volontairement réduit — chaque
dépendance est une surface à auditer/mirrorer dans un registre airgapped :

- `github.com/open-telemetry/opamp-go` — implémentation officielle du
  protocole OpAMP ; réimplémenter le protocole à la main serait un risque
  de sécurité bien plus grand qu'une dépendance de plus.
- `modernc.org/sqlite` — driver SQLite pur Go (sans cgo), pour garder un
  binaire statique et éviter toute dépendance libc dans l'image finale.
- `github.com/prometheus/common/expfmt` + `client_model` — parsing du
  format Prometheus pour le scraping de métriques optionnel ; préféré à un
  parseur fait main pour ce format (risques de bugs de parsing sur une
  entrée non totalement fiable).
- `gopkg.in/yaml.v3`, `github.com/google/uuid` — utilitaires standards,
  largement utilisés et maintenus.

Aucune dépendance HTTP framework (Gin, Echo, chi…) : le `net/http` de la
bibliothèque standard suffit pour la surface de ce projet, donc n'a pas été
ajouté.

## Dépendances UI (npm)

```
npm audit
```

Résultat : **0 vulnérabilité** après avoir fixé `vite` à `^6.3.5` (la
version 5.x embarquait une version d'`esbuild` avec un CVE modéré affectant
uniquement son serveur de développement, jamais le bundle de production
livré). Ré-exécutez `npm audit` après tout changement de dépendance.

## Scan d'image de conteneur

Non exécuté ici (pas de démon Docker disponible dans cet environnement de
développement). **À ajouter en CI** après le build des deux images
(`Dockerfile` et `ui/Dockerfile`), par exemple avec Trivy :

```
trivy image opamp-fleet-server:latest
trivy image opamp-fleet-ui:latest
```

Les deux images sont construites pour minimiser ce qu'il y a à scanner :
binaire Go statique (`CGO_ENABLED=0`), base `distroless/static-debian12`
(pas de shell, pas de gestionnaire de paquets, pas de libc), utilisateur
non-root fixe. Voir `Dockerfile` et `ui/Dockerfile`.

## Modèle RBAC Kubernetes

Voir `RBAC.md` : aucun `ClusterRole` nulle part dans ce dépôt, permissions
par défaut nulles côté serveur, opt-in namespace-par-namespace pour toute
extension future.

## Authentification

Deux jeux de jetons bearer **séparés** — un pour les connexions OpAMP
(collecteurs), un pour l'API REST (UI/opérateurs), voir
`AgentAuthTokensFile`/`APIAuthTokensFile` dans `internal/config/config.go`.
Le serveur refuse de démarrer si les deux pointent vers le même fichier.
Cette séparation a été ajoutée après une première revue de sécurité de ce
dépôt : dans la version initiale, un seul jeu de jetons servait aux deux
canaux, ce qui voulait dire qu'un collecteur compromis — qui n'a besoin que
d'un jeton pour ouvrir une connexion OpAMP — pouvait aussi appeler l'API
REST et pousser une configuration arbitraire vers *tous* les autres agents
de la flotte. C'était une élévation de privilèges réelle, maintenant
fermée par construction (deux `TokenVerifier` distincts, chacun ne
connaissant que son propre fichier).

Chaque jeton est chargé depuis un fichier monté depuis un Secret, rechargé
périodiquement sans redémarrage (`internal/auth/tokens.go`), comparé en
temps constant (`crypto/subtle.ConstantTimeCompare`) sur son empreinte
SHA-256 plutôt que sa valeur brute.

Les tentatives échouées sont journalisées (jamais la valeur du jeton) et
limitées en débit par IP source (`internal/ratelimit` :
10 échecs/minute par défaut, au-delà : `429 Too Many Requests`, aussi bien
côté OpAMP que côté API) — avant cette revue, un échec d'authentification
était invisible et sans coût, ce qui rendait un brute-force silencieux et
gratuit.

Ce n'est toujours pas un système de comptes utilisateurs : voir la note
dans `internal/api/agents.go` (`resolvePushedBy`) sur la limite
d'imputabilité par utilisateur sans proxy d'authentification devant l'API.

## Durcissement du listener OpAMP

`opamp-go` (bibliothèque officielle utilisée pour le protocole) construit,
via sa méthode `Server.Start()`, son propre `http.Server` interne **sans
aucun `ReadHeaderTimeout`** (vérifié dans le source de la v0.23.0) — ce qui
expose le listener OpAMP à une attaque de type slowloris. Ce projet
n'utilise donc pas `Start()` : `internal/opampserver/httpserver.go` utilise
`Server.Attach()` pour récupérer le handler et construit son propre
`http.Server` avec `ReadHeaderTimeout` défini. Attention en y touchant :
`ReadTimeout`/`WriteTimeout` ne sont volontairement PAS définis sur ce
serveur, puisqu'ils s'appliqueraient à toute la durée de vie de la
connexion — y compris après upgrade WebSocket — et couperaient de force
chaque agent au bout du délai.

Le transport HTTP-plain de secours d'OpAMP (agents qui n'utilisent pas de
WebSocket) est borné à 1 Mio via `http.MaxBytesReader`
(`maxPlainHTTPBodyBytes`).

## Limite connue, non corrigeable sans forker `opamp-go`

`opamp-go` v0.23.0 n'impose **aucune limite de taille** sur les messages
WebSocket reçus (`wsConn.ReadMessage()` sans `SetReadLimit`), et
n'expose aucun moyen de configurer cette limite depuis l'extérieur de la
bibliothèque. Un agent qui détient un jeton valide (donc déjà un
collecteur légitime ou compromis) pourrait en théorie envoyer un message
excessivement volumineux et faire grossir la mémoire du serveur.

Mitigation actuellement en place, suffisante mais pas idéale : la limite
mémoire du pod (`resources.limits.memory: 512Mi` dans
`deploy/k8s/platform/07-deployment.yaml`) — un abus se traduit par un
`OOMKilled` et un redémarrage du pod, jamais par un impact au niveau du
nœud ou du cluster. À faire si ce point devient critique pour vous :
proposer un correctif à `open-telemetry/opamp-go` pour exposer
`SetReadLimit`, ou forker le vendoring de la partie WebSocket.

## Content-Security-Policy de l'UI et `style-src 'unsafe-inline'`

`cmd/fleet-ui-server` sert la CSP la plus stricte possible sauf sur un
point : `style-src` inclut `'unsafe-inline'`, nécessaire parce que les
composants React (`ui/src/components/*.tsx`) utilisent largement des
styles inline (`style={{...}}`) plutôt que des classes CSS. Sans ce
relâchement, le navigateur bloquerait ces styles. Ce n'est pas un vecteur
XSS en soi (aucune donnée utilisateur n'atteint ces attributs `style` --
tout le rendu passe par JSX, qui échappe automatiquement), mais ça
affaiblit la CSP par rapport à l'idéal (aucune exécution/injection inline
d'aucune sorte). Correctif possible mais non fait ici (effort/bénéfice
jugé faible face aux autres points) : migrer les styles inline vers des
classes dans `ui/src/styles.css`, puis retirer `'unsafe-inline'` de
`style-src` dans `cmd/fleet-ui-server/main.go`.
