KRAKEND_LOCAL_IP ?= 192.168.1.12

build:
	go build -o srv ./server
.PHONY: build

image:
	docker build -f ./server/Dockerfile -t krakend-otel-example:latest ..

conf: 
	cat ./docker_compose/conf.local/prometheus/prometheus.env_tmpl.yml | envsubst > ./docker_compose/conf.local/prometheus/prometheus.yml

docker_up:
	cd ./docker_compose && docker-compose -p krakend_otel -f compose.yaml up -d 

docker_down:
	cd ./docker_compose && docker-compose -p krakend_otel -f compose.yaml down --remove-orphans -v

docker_local_up:
	cd ./docker_compose && docker-compose -p krakend_otel_local -f compose.local.yaml up -d 

docker_local_down:
	cd ./docker_compose && docker-compose -p krakend_otel_local -f compose.local.yaml down --remove-orphans -v
	
run_local_up: srv
	cd ./docker_compose && docker-compose -p krakend_otel_local -f compose.local.yaml up -d 
	./srv -c ./docker_compose/conf.local/krakend_front/configuration.json  > log_frontend.log &
	./srv -c ./docker_compose/conf.local/krakend_middle/configuration.json  > log_middle.log &
	./srv -c ./docker_compose/conf.local/krakend_back/configuration.json  > log_back.log

run_local: srv
	./srv -c ./docker_compose/conf.local/krakend_front/configuration.json  > log_frontend.log &
	./srv -c ./docker_compose/conf.local/krakend_middle/configuration.json  > log_middle.log &
	./srv -c ./docker_compose/conf.local/krakend_back/configuration.json  > log_back.log

kill:
	cd ./docker_compose && docker-compose -p krakend_otel_local -f compose.local.yaml down
	@ps -aux | grep 'srv.*krakend' | grep -v 'grep' | sed "s/[[:space:]]\+/ /g" | cut -d ' ' -f 2 | xargs kill

kill_srv:
	@ps -aux | grep 'srv.*krakend' | grep -v 'grep' | sed "s/[[:space:]]\+/ /g" | cut -d ' ' -f 2 | xargs kill

dbg:
	dlv debug ./server --wd . -- -p 7979 -c ./krakend.json
.PHONY: dbg
