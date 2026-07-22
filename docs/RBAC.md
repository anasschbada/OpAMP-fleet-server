# Modèle RBAC : zéro ClusterRole, permissions par namespace, opt-in explicite

Ce document explique le modèle de permissions Kubernetes du serveur OpAMP, conçu
pour un cluster **sécurisé, airgapped, sans accès admin**. Il fixe une règle
dure : **aucune ressource de ce dépôt ne définit ni ne requiert de
`ClusterRole`/`ClusterRoleBinding`.**

## 1. Ce que le serveur OpAMP n'a PAS besoin de faire

Le protocole OpAMP est **initié par l'agent** : chaque collecteur OpenTelemetry
ouvre une connexion sortante (WebSocket) vers le serveur et lui envoie tout ce
dont l'UI a besoin :

- `AgentDescription` : attributs identifiants (`service.name`,
  `k8s.namespace.name`, `k8s.pod.name`, `k8s.node.name`, version du collecteur…)
  — définis par le `resourcedetection`/`k8sattributes` processor **côté
  collecteur**, pas lus depuis l'API Kubernetes par le serveur.
- `EffectiveConfig` : la config YAML réellement chargée.
- `Health` : statut, uptime.
- `RemoteConfigStatus` : APPLIED / FAILING, après un push de config.

Conséquence directe : **le registre de flotte, le regroupement par namespace,
le statut de santé et le poussage de configuration ne nécessitent aucun appel
à l'API Kubernetes.** Le serveur peut tourner avec un `ServiceAccount` sans
aucun droit RBAC (`automountServiceAccountToken: false` même), et fonctionner
identiquement dans un cluster où vous n'avez aucun accès admin.

C'est le choix par défaut de ce projet.

## 2. Pourquoi aucun `ClusterRole` n'est même envisagé

Rappel du fonctionnement RBAC de Kubernetes, qui rend `ClusterRole` non
seulement évitable mais **structurellement inutile** ici :

- Un `Role` (+ `RoleBinding`) ne peut accorder l'accès qu'à des ressources
  **namespaced** (Pods, ConfigMaps, Deployments, Secrets, etc.) dans **un seul
  namespace**.
- Les ressources **non-namespaced** (`Node`, `Namespace`, `PersistentVolume`,
  `ClusterRole`…) ne peuvent être autorisées que via un `ClusterRole` lié par
  un `ClusterRoleBinding` (ou un `RoleBinding` référençant un `ClusterRole`, ce
  qui reste inefficace pour une ressource non-namespaced). Un `Role` seul ne
  peut **jamais** les exposer, quel que soit son contenu.
- Le serveur n'a besoin de lister ni les `Namespace`, ni les `Node` : les
  namespaces affichés dans l'UI sont simplement ceux que les agents connectés
  rapportent eux-mêmes (agrégation applicative), pas une liste énumérée via
  l'API Kubernetes.

Résultat : il n'existe **aucun besoin fonctionnel** justifiant un
`ClusterRole` dans ce projet, indépendamment de la contrainte de sécurité
imposée. La contrainte et l'architecture s'alignent naturellement.

## 3. Si un jour vous voulez une vérification croisée Kubernetes (non implémenté, juste le patron)

**Le serveur livré dans ce dépôt ne fait aucun appel à l'API Kubernetes,
point final** — il n'y a pas de fonctionnalité "cross-check" cachée derrière
un flag désactivé : elle n'existe pas dans le code, volontairement, pour ne
rien livrer de partiellement fait.

Si un besoin futur apparaît (ex. afficher le nom réel du pod/nœud tel que vu
par l'API, détecter un pod OOMKilled, lire une `ConfigMap` de référence),
voici le patron à suivre pour rester dans les limites de ce document — sans
jamais introduire de `ClusterRole` :

### Le modèle d'opt-in : RoleBinding cross-namespace, sans admin cluster

Le point clé qui permet ce modèle **sans accès cluster-admin** : un
`RoleBinding` vit dans le namespace cible (ex. `equipe-a`) et peut désigner
comme sujet un `ServiceAccount` d'un **autre** namespace (celui du serveur
OpAMP, ex. `opamp-system`). Ni la création de ce `RoleBinding`, ni le `Role`
associé, ne requièrent de toucher à des ressources cluster-scoped — ce sont
des ressources namespaced ordinaires, créables par **le titulaire de
`equipe-a` lui-même**, sans qu'un cluster-admin intervienne.

```
Namespace: equipe-a                    Namespace: opamp-system
┌─────────────────────────────┐        ┌──────────────────────────┐
│ Role: opamp-readonly         │        │ ServiceAccount:           │
│  - get/list/watch pods       │        │   opamp-server            │
│  - get/list configmaps       │        │                           │
│                               │        │ Deployment: opamp-server  │
│ RoleBinding: opamp-server-... │◄───────┤   (utilise cette SA)      │
│  subject: ServiceAccount      │ bind   │                           │
│    opamp-server (ns:          │        │                           │
│    opamp-system)              │        │                           │
└─────────────────────────────┘        └──────────────────────────┘
```

Chaque équipe applique (ou refuse) elle-même le manifeste
`deploy/k8s/namespace-onboarding/role-and-binding.yaml` (template fourni,
namespace et nom de SA à substituer) dans son propre namespace. Aucun droit
cluster-wide n'est jamais accordé au serveur, même cumulé sur N namespaces
onboardés : chaque `RoleBinding` reste local à son namespace, listable et
révocable indépendamment (`kubectl get rolebindings -n equipe-a`).

Le jour où une fonctionnalité serveur consomme réellement ce Role (ex. un
appel `client-go` en lecture seule), le code applicatif doit maintenir sa
propre liste blanche des namespaces pour lesquels tenter l'appel, et traiter
un 403 (RoleBinding absent dans ce namespace) comme "fonctionnalité
indisponible ici", jamais comme une erreur fatale.

## 4. Ce que `deploy/k8s/platform/` contient (et ne contient pas)

| Fichier | Contenu | Scope RBAC |
|---|---|---|
| `01-serviceaccount.yaml` | ServiceAccount du serveur OpAMP | aucun (`automountServiceAccountToken: false`, pas de Role/RoleBinding) |
| `10-ui-serviceaccount.yaml` | ServiceAccount de l'UI | aucun, idem |
| `07-deployment.yaml`, `11-ui-deployment.yaml` | Les deux workloads | référencent leur ServiceAccount respective, aucune n'a de droit |
| — | **Aucun `ClusterRole`, aucun `ClusterRoleBinding`, aucun `Role`, aucun `RoleBinding` nulle part dans ce dépôt.** Le patron de la section 3 n'est qu'un modèle documenté pour un besoin futur — rien à côté ne l'active aujourd'hui. | — |

## 5. Résumé pour un audit sécurité

- Surface RBAC par défaut : **nulle** (aucun droit sur l'API Kubernetes).
- Surface RBAC maximale (tous les opt-in activés sur tous les namespaces) :
  lecture seule (`get/list/watch`) de `pods` et `configmaps`, jamais
  d'écriture, jamais de `secrets`, jamais de ressource cluster-scoped.
- Aucune dépendance sur un `ServiceAccount` avec `cluster-admin` ou
  équivalent, à aucune étape de l'installation ou de l'exploitation.
- Le push de configuration vers un collecteur passe par le canal OpAMP
  (WebSocket applicatif), jamais par une modification de `ConfigMap` ou un
  redémarrage de pod via l'API Kubernetes.
