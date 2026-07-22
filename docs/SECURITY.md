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

Un jeu de jetons bearer partagés (fichier monté depuis un Secret,
rechargé périodiquement sans redémarrage — voir
`internal/auth/tokens.go`), comparés en temps constant
(`crypto/subtle.ConstantTimeCompare`) sur leur empreinte SHA-256 plutôt que
la valeur brute. Ce n'est pas un système de comptes utilisateurs : voir la
note dans `internal/api/agents.go` (`resolvePushedBy`) sur la limite
d'imputabilité par utilisateur sans proxy d'authentification devant l'API.
