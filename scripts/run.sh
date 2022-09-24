#!/bin/bash
# test in localhost
./vguard -id=$1 -r=$2 -s=true

# run in cluster
#./vguard -id=$1 -log=4 -r=$2 -s=true -cfp=./config/cluster_4.conf
