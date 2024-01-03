# How to run the example

In the `docker_compose` folder there is the environment to run the observability software.

## Run the dockerized example:

```
make image
cd docker-compose && make up
```

## Run local krakend instances and other services in docker compose

Since the docker-compose environment runs in its own network, in order to reach
your local running services, you need to provide it with your network visible ip:
you might have for example `localhost` that maps to `127.0.0.1` and an IP
assigned to your ethernet interface, that could be `192.168.1.12` for example. The
localhost one will not when used from a container, however, the other one can
be used to gather data from your running services (that is needed to scrape
prometheus metrics).

So, you should run:

```
export KRAKEND_LOCAL_IP="{your non localhost machine ip}"
make conf
make run
```

# What do you get

By running the example, you spawn 3 servers based on the Lura framework (we could
say the example is a super simplified KrakenD CE).

- `krakend_front`: the one to receive the requests
- `krakend_middle`: a middle krakend to check how traces are propagated
- `krakend_back`: a back krakend that makes request to `https://jsonplaceholder.typicode.com` to
    get some dummy data.
    
The config files can be found in:

- [./docker_compose/conf/krakend_*](./docker_compose/conf/) directories for the dockerized version of the example.
- [./docker_compose/conf.local/krakend_*](./docker_compose/conf.local/) directories for running
    the krakends in the local environment.


# Execute some requests

There is a bash script to make some requests, that you can run with:

```bash
bash ./make_request.sh
```

(You might want to edit the script to point to the dockerized version of the krakend frontend
server or the local one).

# Access the dashboards

In both, the dockerized and the local options, you will end with some containers running some
services that you can use to check the metrics / traces:

- Grafana: at http://localhost:3000 , with the username: `krakend` and the password: `krakend` as
    admin users
- Jaeger: at http://localhost:16686.
