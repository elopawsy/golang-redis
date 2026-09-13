# WasmRedis

Un projet pour apprendre Go : un stockage clé-valeur persistant, des recherches
typées et un tableau de bord React pour suivre des files de tâches.

Les cinq phases du moteur sont terminées : commandes, persistance, batch, index
et expiration. La [roadmap](CLAUDE.md) décrit les choix et le périmètre.

## Démarrer

Prérequis : Go 1.26.3+ et Node.js 22.12+ ou une version LTS plus récente.

```bash
npm ci --prefix web
npm run build --prefix web
go run . -demo
```

Ouvrir **http://127.0.0.1:8080**. Le paramètre `-demo` ajoute trois files de
1 000 tâches. Sans ce paramètre, les files sont vides. Ctrl-C arrête le serveur
et sauvegarde les écritures clé-valeur en attente.

Le stockage clé-valeur est sauvegardé dans `data/`. Les **files de tâches du
tableau de bord restent en mémoire** et disparaissent à l'arrêt.

Pour développer React, garder le serveur Go dans un terminal et lancer
`npm run dev --prefix web` dans un autre. Ouvrir alors **http://127.0.0.1:5173**.
Le proxy Vite transmet les appels API au port 8080.

## Terminal et commandes

```bash
go run . -cli
```

```text
> SET nom "Ada Lovelace"
OK
> SET age 42
OK
> SET etiquette "42"
OK
> GET age
42
> SET code secret EX 10
OK
> GET WHERE value >= 21
[{"key":"age","value":"42","isNumber":true,"expiresAt":"0001-01-01T00:00:00Z"}]
> DEL code
OK
> PING
PONG
> EXIT
```

| Commande | Rôle |
| --- | --- |
| `SET clé valeur [EX secondes]` | Créer ou remplacer une valeur et son expiration |
| `GET clé` | Lire une valeur |
| `DEL clé` ou `DELETE clé` | Supprimer une clé |
| `PING` | Répondre PONG |
| `GET WHERE key opérateur valeur` | Chercher sur les clés |
| `GET WHERE value opérateur valeur` | Chercher sur les valeurs |
| `EXIT`, `QUIT` ou Ctrl-D | Quitter le terminal |

Opérateurs : `equals`, `contains`, `>`, `>=`, `<`, `<=`.
Les commandes sont insensibles à la casse ; les clés et valeurs y sont sensibles.
Les résultats de recherche sont triés par clé.

`SET k 42` stocke un nombre ; `SET k "42"` et `SET k mot` stockent des chaînes.
Les nombres utilisent float64 : les très grands entiers peuvent perdre de la
précision. Les valeurs non finies, comme NaN et Inf, restent des chaînes.

Une plage numérique ne renvoie que des nombres. Une plage entre guillemets compare
uniquement les chaînes dans l'ordre lexicographique. `contains` recherche dans
les chaînes, pas dans les nombres. Les clés sont toujours des chaînes.

Les guillemets simples ou doubles conservent les espaces ; les échappements ne
sont pas interprétés. `EX` attend un entier positif en secondes. Un SET sans EX
applique le TTL par défaut, qui vaut zéro (aucune expiration) sauf configuration.

## Persistance et configuration

```text
SET / DEL → mémoire et index → buffer → journal AOF
snapshot → fichier temporaire synchronisé → renommage → journal vidé
redémarrage → snapshot + journal → suppression des expirés → index reconstruits
```

Le journal est synchronisé sur disque au flush. Un snapshot compact est produit
périodiquement. Ses numéros de séquence empêchent de rejouer d'anciennes écritures
après une compaction interrompue. Une dernière ligne de journal incomplète est
retirée au redémarrage ; une corruption sur une ligne complète bloque l'ouverture.

Un arrêt normal sauvegarde les opérations restantes. Un arrêt brutal peut perdre
les opérations encore dans le buffer, depuis le dernier flush réussi. L'API
`POST /api/flush` permet d'attendre explicitement leur enregistrement sur disque.

| Variable | Défaut |
| --- | --- |
| `WASMREDIS_FLUSH_INTERVAL` | `1s` |
| `WASMREDIS_SNAPSHOT_INTERVAL` | `2m` |
| `WASMREDIS_DEFAULT_TTL` | `0s` |
| `WASMREDIS_EXPIRY_SCAN_INTERVAL` | `10s` |
| `WASMREDIS_BTREE_ORDER` | `32` (degré minimum, au moins 2) |
| `WASMREDIS_AOF_PATH` | `data/aof.log` |
| `WASMREDIS_SNAPSHOT_PATH` | `data/snapshot.json` |

Le fichier `.env.example` fournit ces valeurs ; il n'est pas lu automatiquement.

```bash
export WASMREDIS_FLUSH_INTERVAL=500ms
go run .
```

Les valeurs mal formées utilisent les défauts. Les valeurs numériques hors des
limites permises sont refusées au démarrage. Une clé expirée est supprimée à la
lecture et par le nettoyage périodique, avec mise à jour des index.

Un seul processus peut ouvrir les mêmes fichiers : le terminal et le serveur
utilisent un verrou système. Arrêter l'un avant de lancer l'autre sur les mêmes
données. `go run . -memory` ou `go run . -cli -memory` désactive la persistance.
`-addr 127.0.0.1:9090` permet de choisir un autre port HTTP.

## API clé-valeur et batch

| Méthode | Route | Corps |
| --- | --- | --- |
| POST | `/api/command` | `{"command":"SET age 42"}` |
| POST | `/api/batch` | `{"commands":["SET age 42","GET WHERE value >= 21"]}` |
| POST | `/api/flush` | Aucun |
| POST | `/api/snapshot` | Aucun |

Le batch exécute les commandes dans l'ordre et renvoie une réponse par commande.
Une erreur ne stoppe pas les suivantes. Ce n'est pas une transaction : les autres
clients peuvent intervenir entre deux commandes. La requête accepte au maximum
1 000 commandes et 16 Kio de JSON.

Une réponse contient `value`, `isNumber` et `entries` ; les résultats de recherche
vides utilisent `entries: []`. Les erreurs du batch ont un champ `error`.
Une commande invalide seule renvoie HTTP 400. Un échec de flush ou de snapshot
renvoie HTTP 500. Les recherches ne sont pas paginées : elles renvoient toutes
les correspondances.

## Files de tâches et interface React

1. Créer une file avec un nom simple, par exemple `emails`.
2. Ajouter une tâche : son contenu est du texte.
3. Démarrer la suivante : la première tâche en attente passe en cours.
4. Terminer la tâche ou la marquer en erreur.
5. Réessayer remet une tâche en erreur en attente, à sa place d'origine.

Les statuts sont `pending`, `running`, `completed` et `failed`.
Le traitement est manuel : aucun worker n'exécute le contenu.
Les tâches terminées restent consultables. Les filtres et compteurs se
rafraîchissent toutes les deux secondes.

| Méthode | Route | Corps / paramètres |
| --- | --- | --- |
| GET | `/api/queues` | Noms et compteurs |
| POST | `/api/queues` | `{"name":"emails"}` |
| GET | `/api/queues/emails/tasks` | `?offset=0&limit=100&status=pending` |
| POST | `/api/queues/emails/tasks` | `{"payload":"Envoyer un email"}` |
| POST | `/api/queues/emails/claim` | Démarrer la première tâche en attente |
| PATCH | `/api/queues/emails/tasks/1` | `{"status":"completed"}`, `failed` ou `pending` selon l'état |

Les pages contiennent `items`, `total` et `offset`. La limite est comprise entre
1 et 200. Un contenu de tâche peut faire au maximum 4 096 octets.
Une entrée invalide renvoie 400, un élément absent 404 et un conflit 409.

Le défilement virtuel utilise des lignes de 80 pixels. Un bloc prend la hauteur
totale de la liste ; seules les lignes visibles et quelques lignes de marge sont
rendues. Au maximum 200 tâches sont chargées autour de la position du défilement.

## Organisation du code

```text
main.go                       configuration et cycle de vie
cli.go                        terminal
server.go                     serveur et démonstration
internal/command/types.go     types des commandes
internal/command/command.go   parser
internal/engine/types.go      structures du moteur
internal/engine/status.go     statuts des tâches
internal/engine/engine.go     SET, GET, DEL et exécution
internal/engine/persistence.go journal, snapshot et restauration
internal/engine/background.go nettoyage et tâches périodiques
internal/engine/batch.go      commandes en lot
internal/engine/query.go      GET WHERE
internal/engine/queues.go     files de tâches
internal/storage/             fichiers, types et verrous
internal/index/               index inversé et B-Tree
internal/config/              réglages et types
internal/api/                 routes et réponses HTTP
web/src/                      tableau de bord React
```

Le code ne contient pas de commentaires. Les explications restent dans cette
documentation. Pour commencer, lire `Engine.Set`, `Get`, puis `Execute`.

Deux bibliothèques Go évitent d'allonger le code :
[google/btree](https://github.com/google/btree) pour l'arbre ordonné et
[gofrs/flock](https://github.com/gofrs/flock) pour les verrous de fichiers.

## Vérifier

```bash
go test -race ./...
go vet ./...
go build ./...
npm run build --prefix web
cd web
npx playwright install chromium
npm run test:e2e
```

Les tests couvrent le parser, les types, la restauration, les erreurs disque,
la compaction interrompue, les batchs, les index, l'expiration et la concurrence.
Les tests navigateur lancent un serveur isolé en mémoire sur le port 8081.

## Suite distincte

WebAssembly, OPFS, worker navigateur et SDK TypeScript appartiennent aux lots
ultérieurs évoqués dans le plan initial. Ils ne font pas partie des cinq phases
du moteur terminées ici. Il n'y a pas encore de protocole Redis réseau,
d'authentification, de persistance des files ou de worker qui exécute les tâches.
