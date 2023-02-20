## V-Guard configuration file
The first line of the config file is left for memo.

    0 127.0.0.1 vg 10000 10001 10002 10003 10100 10200 10300 10007

`0` is the server/vehicle's ID #Note that `0` is the proposer and `1` is the pivot validator\
`127.0.0.1` is the server/vehicle's IP address\
`vg` is the program identifier (obsolete)

To obtain high performance and fairness in handling networking messages, 
V-Guard establishes a connection for each consensus phase.

`10000` is the proposer's TCP listener port number for ordering phase A\
`10001`is the proposer's TCP listener port number for ordering phase B\
`10002`is the proposer's TCP listener port number for consensus phase A\
`10003`is the proposer's TCP listener port number for consensus phase B\
`10100`is the validator's dialog TCP port number for ordering phase A\
`10200`is the validator's dialog TCP port number for ordering phase B\
`10300`is the validator's dialog TCP port number for consensus phase A\
`10007`is the validator's dialog TCP port number for consensus phase B

The `loadconf.go` file shows how configuration is loaded into the program.