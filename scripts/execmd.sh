#!/bin/bash

i=0

read -p "---------- executing commands remotely? --------------"
while IFS= read -r line; do
    echo "executing on server $i -> id: $line"
    ssh $line "cd go/src/vguard; ./scripts/run.sh $((i+1)) 1" &

    # impose latency
    # ssh $line "sudo tc qdisc add dev ens3 root netem delay 10ms" &

    # delete latency
    # ssh $line "sudo tc qdisc del dev ens3 root" &

    # kill vguard process clearing up ports
    # ssh $line "./go/src/popcorn/killproc.sh" &
    echo "server $i -> $line is done"
    ((i++))
done < serverlist