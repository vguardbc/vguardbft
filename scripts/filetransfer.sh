#!/bin/bash

i=0
read -p "---------- transfer files? --------------"
while IFS= read -r line; do
    echo "transferring to replica $i; file: $1; id: $line"
    scp ./$1 $line:/home/ubuntu/go/src/
    ((i++))
done < serverlist