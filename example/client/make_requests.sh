#!/bin/bash

# to generate a new base script:
#
# docker run --rm -i -v ${PWD}:/app \
#     --user $(id -u):$(id -g) \
#     -w /app grafana/k6 \
#     new client.js

docker run --rm -i -v ${PWD}:/app \
    --user $(id -u):$(id -g) \
    -w /app grafana/k6 \
    run k6client/client.js

## for i in {1..1}
## # for i in {1..1000}
## do
##     # curl localhost:54444/fake/fsf
##     # curl localhost:54444/combination/2
##     curl localhost:54444/combination/1
##     # curl localhost:54444/direct/slow
##     sleep 0.1
##     # curl localhost:54444/direct/delayed
##     # curl localhost:54444/direct/drop
## done
## 
## # curl localhost:44444/fake/fsf | jq 
## # echo -e "\n"
## # curl localhost:44444/combination/23 | jq
