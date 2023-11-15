#!/bin/bash

# for i in {1..10}
for i in {1..2}
do
    curl localhost:54444/fake/fsf
    curl localhost:54444/combination/2
done

# curl localhost:44444/fake/fsf | jq 
# echo -e "\n"
# curl localhost:44444/combination/23 | jq
