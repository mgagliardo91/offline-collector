# Offline Collector

Offline Collector is a scraper used to collect [Offline Raleigh](https://get-offline.com/raleigh) events. It uses a proxy-rotator to gather these events to keep from having a single IP Address tracked/blocked.

## Local Setup

This setup requires go version `1.11+`

1. Clone and enter the repository
1. run `go get -u`
1. run `go build`
1. run `OFFLINE_SERVER=<Address> OFFLINE_PORT=<PORT> ./offline-collector`. The `OFFLINE_SERVER` and `OFFLINE_PORT` should point to the running instance of [Offline Server](https://github.com/mgagliardo91/offline-server). Excluding these variables will default to `http://localhost:3000`.

Options:
- `--start`: Used to indicate the start date to collect from (format `YYYY-MM-DD`). Defaults to today
- `--end`: Used to indicate the end date to collect to (format `YYYY-MM-DD`). Defaults to value of `start`.

## Running with Docker

To run with docker, you first have to create the docker image by executing:

```
run `docker build -t offline-collector`
```

Once the image is created, you can create/run the container by executing:

```
docker run \
--env OFFLINE_SERVER=<URL> OFFLINE_PORT=<PORT> \
-d \
--name offline-collector \
offline-collector
```

Leave of `-d` if you want to run the container in the same shell (keep it from detaching).

## Running with Docker-Compose

A `docker-compose.yaml` file is also included which will spin up a network, start the server and then kick of the collector.

To start:
```
ELASTIC_SEARCH_URL=<ES_URL> docker-compose up
```

To shutdown:
```
docker-compose down
```

