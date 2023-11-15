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

# Access the dashboards


