# AnnecyBike Tracker

Suivi en temps réel des vélos et stations Velonecy (Annecy), avec historique complet et statistiques par vélo.

## Fonctionnalités

- **Carte en temps réel** — position de chaque vélo et disponibilité des stations, mise à jour toutes les 60s via WebSocket
- **Historique complet** — chaque snapshot (position, batterie, statut) est enregistré en base depuis le démarrage
- **Détection automatique des trajets** — un trajet est créé dès qu'un vélo change de station
- **Statistiques par vélo** — trajets effectués, distance totale, évolution de la batterie, taux de disponibilité
- **Dashboard flotte** — vue globale, trajets par jour, distribution batterie, stations les plus actives

## Prérequis

- [Docker](https://docs.docker.com/get-docker/) + [Docker Compose](https://docs.docker.com/compose/) v2

## Installation

```bash
# 1. Copier le fichier de configuration
cp .env.example .env

# 2. Définir un mot de passe pour la base de données
#    Éditer .env et changer POSTGRES_PASSWORD

# 3. Lancer tous les services
docker compose up -d --build
```

L'application est disponible sur **http://localhost** après environ 30 secondes (le temps que la DB s'initialise et que le premier poll se termine).

## Configuration (.env)

| Variable | Défaut | Description |
|---|---|---|
| `POSTGRES_PASSWORD` | *(requis)* | Mot de passe PostgreSQL |
| `POSTGRES_DB` | `annecybike` | Nom de la base |
| `POSTGRES_USER` | `annecybike` | Utilisateur PostgreSQL |
| `HTTP_PORT` | `80` | Port HTTP exposé |
| `POLL_INTERVAL_SECONDS` | `60` | Intervalle de polling GBFS |

## Architecture

```
nginx (:80)
├── /api/*   → backend Go (Gin, REST API)
├── /ws/*    → backend Go (WebSocket)
└── /*       → frontend React (SPA statique)

poller Go
└── toutes les 60s : fetch GBFS → PostgreSQL
    ├── bike_snapshots    (hypertable TimescaleDB)
    ├── station_snapshots (hypertable TimescaleDB)
    └── trips             (détection automatique)

PostgreSQL + TimescaleDB
└── compression automatique après 7 jours
```

## Pages

| URL | Description |
|---|---|
| `/` | Carte interactive temps réel |
| `/bikes/:id` | Historique et stats d'un vélo |
| `/stations/:id` | Occupation d'une station |
| `/stats` | Statistiques globales de la flotte |

## API REST

Base URL : `http://localhost/api/v1`

```
GET /bikes/live                    Tous les vélos (position actuelle)
GET /bikes/:id/history             Historique positions/batterie
GET /bikes/:id/trips               Trajets d'un vélo
GET /bikes/:id/stats               KPIs d'un vélo
GET /stations/live                 Toutes les stations + disponibilité
GET /stations/:id/history          Occupation d'une station
GET /trips                         Liste des trajets (filtrable)
GET /stats/fleet                   Vue globale flotte
GET /stats/trips-per-day           Trajets par jour
GET /stats/battery-distribution    Distribution de la batterie
GET /stats/busiest-stations        Stations les plus actives
WS  /ws/live                       Push snapshot après chaque poll
```

## Commandes utiles

```bash
# Voir les logs en direct
docker compose logs -f poller
docker compose logs -f backend

# Vérifier que les données arrivent (après 2 min)
docker compose exec db psql -U annecybike -c "SELECT COUNT(*) FROM bike_snapshots;"

# Voir les trajets détectés
docker compose exec db psql -U annecybike -c "SELECT bike_id, start_time, distance_meters FROM trips ORDER BY start_time DESC LIMIT 10;"

# Arrêter sans perdre les données
docker compose down

# Arrêter ET supprimer la base (reset complet)
docker compose down -v
```

## Détection des trajets

Le poller maintient l'état de chaque vélo en mémoire entre deux polls. Un trajet est détecté quand :

- **Départ** : `station_id` passe de non-nul à `null` (vélo décroché)
- **Arrivée** : `station_id` repasse à non-nul (vélo raccroché)

La distance est estimée via la formule de Haversine (vol d'oiseau × 1.3 pour approximer le trajet cycliste réel).

## Source des données

API GBFS v2.2 publique fournie par [Fifteen](https://www.fifteen.eu/) pour le réseau **Velonecy** d'Annecy :
`https://gbfs.partners.fifteen.eu/gbfs/2.2/annecy/en/`
