# Avis sur la prod : ce qui est solide, ce qui manque encore

Évaluation honnête après revue de sécurité, pentest manuel, ajout de tests
et mise en place CI/Helm. Pas un satisfecit — un vrai état des lieux.

## Ce qui est solide

- **Modèle RBAC** : zéro `ClusterRole`/`Role`/`RoleBinding` nulle part dans
  ce dépôt, vérifié à la main et par `kubeconform` sur les manifestes bruts
  et le rendu Helm. Le protocole OpAMP fonctionne intégralement sans accès
  API Kubernetes.
- **Séparation des jetons agent/API** : corrige une élévation de privilèges
  réelle trouvée pendant la revue (un collecteur compromis pouvait pousser
  une config vers toute la flotte). Testé en conditions réelles (curl,
  puis tests automatisés).
- **Surface d'attaque du code** : `gosec` à 0 finding, `gitleaks` sur tout
  l'historique à 0 leak, `npm audit` à 0 vulnérabilité, pentest manuel
  (path traversal, injection SQL, CRLF, corps surdimensionnés, YAML
  malformé, CORS, rate limiting) sans faille trouvée à part un bug mineur
  (500 au lieu de 400 sur un octet nul) déjà corrigé et couvert par un test
  de non-régression.
- **Images conteneur** : binaires Go statiques, base `distroless` sans
  shell ni libc, utilisateur non-root fixe, aucune dépendance nginx/OS à
  patcher.
- **Tests automatisés** : suite de tests unitaires/HTTP couvrant
  l'authentification, le rate limiting, la validation des configs,
  l'injection, le stockage (mémoire + SQLite via la même interface), le
  parsing des métriques — voir la couverture par package dans la sortie de
  `go test ./... -cover`.
- **CI** : build/vet/fmt/test/gosec/govulncheck/npm audit/trivy/kubeconform
  sur chaque push, plus Dependabot pour les mises à jour de dépendances.

## Ce qui manque encore pour une vraie prod à enjeux critiques

Classé par impact, pas par ordre d'implémentation :

### 1. Pas de haute disponibilité (le plus important)
SQLite est mono-écrivain : `replicas: 1` imposé, `strategy: Recreate`. Une
panne de nœud = coupure le temps du reschedule (généralement quelques
dizaines de secondes à quelques minutes selon votre cluster). Pas
acceptable si vous avez un SLA de continuité de service sur la gestion de
flotte elle-même (notez : les collecteurs continuent de fonctionner avec
leur dernière config connue pendant la coupure — c'est la gestion/UI qui
est indisponible, pas la télémétrie déjà en place).
**Chemin d'amélioration** : `store.Store` est déjà une interface
(`internal/store`) — ajouter une implémentation Postgres/MySQL derrière la
même interface est le chemin naturel pour passer à plusieurs répliques
sans toucher au reste du code.

### 2. Pas d'authentification par utilisateur / SSO
Un seul jeu de jetons bearer partagés côté API — pas de compte, pas de
RBAC applicatif, pas d'intégration OIDC. `resolvePushedBy` (l'attribution
d'un push de config à un humain précis) dépend entièrement d'un proxy
d'authentification externe non fourni ici. Acceptable pour une petite
équipe de plateforme de confiance ; pas pour une organisation qui a besoin
d'un vrai audit trail par utilisateur ou de permissions différenciées
(lecture seule vs push de config, par exemple).

### 3. Pas de tests d'intégration bout-en-bout automatisés
Les tests unitaires couvrent la logique ; `cmd/opamp-server` (le vrai
`main()`, les deux listeners HTTP réels, l'arrêt propre) a été vérifié à
la main plusieurs fois pendant cette session (curl, client OpAMP jetable)
mais n'a **aucun test automatisé** qui le rejoue en CI. Un changement
futur pourrait casser le câblage réel sans que la CI le détecte.

### 4. Limite de taille des messages WebSocket non corrigeable
Documenté dans `docs/SECURITY.md` : `opamp-go` v0.23.0 n'expose aucun moyen
de borner la taille d'un message WebSocket. Mitigation actuelle (limite
mémoire du pod → OOMKill) suffisante pour un incident isolé, pas pour un
abus soutenu. Nécessite soit un correctif upstream, soit un fork du
transport WebSocket.

### 5. TLS optionnel, pas forcé
Le serveur démarre en clair par défaut avec juste un avertissement de log.
C'est un choix assumé (delegation à un mesh/ingress), mais rien n'empêche
un déploiement en clair *sans* mesh par erreur -- pas de garde-fou
applicatif qui refuserait de démarrer sans TLS ni variable d'environnement
explicite "j'assume le clair".

### 6. Pas de scan d'image exécuté (seulement configuré)
`gosec`/`gitleaks`/`npm audit` ont tourné réellement dans cette session.
Trivy (scan d'image conteneur) est câblé dans la CI mais n'a **jamais
tourné** faute de démon Docker dans cet environnement de développement —
la première exécution en CI sera la première fois que ces images sont
réellement scannées.

### 7. Observabilité du serveur lui-même
Logs structurés JSON présents, mais pas de métriques Prometheus exposées
par le serveur *sur lui-même* (uniquement pour les collecteurs qu'il
supervise) -- si vous voulez superviser la santé du fleet-server avec
votre propre stack d'observabilité, il faudra l'instrumenter.

### 8. Pas de vraie stratégie de sauvegarde du registre SQLite
La PVC persiste les données, mais rien n'automatise un snapshot/backup
régulier de `fleet.db` -- une perte de volume (rare mais possible) reperd
tout l'historique de configuration et le registre d'agents (qui se
reconstruit vite depuis les agents vivants, mais l'historique des push,
lui, ne revient pas).

## Verdict

**Utilisable en production pour une équipe de plateforme interne, sur un
cluster qu'elle contrôle, avec un niveau de confiance déjà établi entre
équipes** (pas d'exposition publique, pas de multi-tenant hostile) — le
modèle RBAC, la séparation des jetons, le durcissement conteneur et la
suite de tests le permettent raisonnablement.

**Pas encore prêt sans travail supplémentaire pour** : un environnement
multi-tenant avec équipes qui ne se font pas confiance (pas d'auth par
utilisateur), un SLA de continuité de service sur la gestion de flotte
elle-même (pas de HA), ou un contexte réglementaire exigeant un audit
trail nominatif complet.

Les points 1 et 2 ci-dessus sont, dans cet ordre, ce que je referais si le
projet devait grandir au-delà de son usage actuel.
