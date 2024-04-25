#!/bin/bash

for i in {1..1000}
do
    # curl localhost:54444/fake/fsf
    # curl localhost:54444/combination/2
    echo -e "\n/__stats/"
    curl localhost:54444/__stats/
    echo -e "\n/__health"
    curl localhost:54444/__health
    echo -e "\n/__echo/"
    curl localhost:54444/__echo/
    # uncomment this to see real 404 not found error codes
    # echo -e "\n/this_does_not_exist/"
    # curl localhost:54444/this_does_not_exist/
    echo -e "\n----\n"
    sleep 1
done
