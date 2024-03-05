#!/bin/bash

curl -X POST --header "Content-Type: application/json" \
    http://krakend:krakend@localhost:53000/api/datasources \
    -d @prometheus_datasource.json 

curl -X POST --header "Content-Type: application/json" \
    http://krakend:krakend@localhost:53000/api/datasources \
    -d @tempo_datasource.json 

curl -X POST --header "Content-Type: application/json" \
    http://krakend:krakend@localhost:53000/api/datasources \
    -d @loki_datasource.json 
