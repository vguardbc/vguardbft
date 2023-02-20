#!/bin/bash
# test in localhost
./vginstance -id=$1 -r=$2 \
-b=1 \
-boo=4 \
-c=6 \
-cfp="./config/cluster_localhost.conf" \
-ci=500 \
-cw=10 \
-d=0 \
-ed=1 \
-gc=false \
-lm=100 \
-log=4 \
-m=32 \
-ml=10000 \
-pm=false \
-s=true \
-w=1 \
-yc=0

# To run in cluster, change -cfp to the cluster config file.
# E.g., -cfp="./config/cluster_4.conf"
