<h1 align="center"> V-Guard: A Fast, Dynamic, and Versatile Permissioned Blockchain Framework for V2X Networks </h1>

## About this repository

*This repository contains the code of V-Guard's core consensus module, the evaluation result of which has been shown in the submission. The code of the storage and gossip modules will be published after the review process.*

V-Guard is a new permissioned blockchain framework that is able to operate 
under dynamic (unstable) memberships; it targets the problem in V2X networks where 
vehicles are often intermittently connected on the roads.

### Member roles
In V-Guard, there are two types of members, vehicles and their manufacturers, operating in two roles: **proposer** and **validator**.

    # Proposer: the vehicle that initiates a V-Guard instance.
    # Validator: 
    #   (1) pivot validator: the proposer's manufacturer(s).
    #   (2) vehicle validator: the other vehicles that participate in this V-Guard instance.

There is no difference between pivot validator(s) and vehicle validators in the voting process; i.e., their votes have the same weight in deciding whether an agreement for an requested action can be committed or not.

### One proposer, one V-Guard
V-Guard is lightweight. Each vehicle creates a V-Guard instance for its own data and operates as proposer.
Thus, each V-Guard instance has only one proposer, which is the vehicle who created this V-Guard instance.
Since date produced by different vehicles is transactionally unrelated, 
their consensus services do not need to intertwine in a single consensus instance. 

Therefore, **there is no need for V-Guard to apply view changes**. 
V-Guard's proposer is a combined role of leader and client compared to other BFT algorithms.
If the proposer fails, no data will be produced, and its consensus service simply halts.
This single failure has no effect on other vehicles' instances (if any). 

### Booth -- membership descriptor 
V-Guard allows consensus to be reached in different memberships (sets of vehicles), namely **booths**. 
A booth must include the proposer (V<sub>P</sub>), pivot validator (V<sub>π</sub>), and at least two other members (vehicles).
An illustration of constructing booths is shown below.


![](./docs/booths.gif)

## Try the Current Version

### Install dependencies
This project uses two external packages for threshold signatures and logging.

    # threshold signatures
    go get github.com/dedis/kyber/
    # logging
    go get github.com/sirupsen/logrus

### Run V-Guard instances locally
Below shows an example of running a four-vehicle V-Guard system consisting of one proposer and three validators 
(one validator as consensus pivot and the other two validators are vehicle validators enrolled in the current quorum)
    # run a proposer
    ./scripts/run.sh 0 0

    # run validators
    ./scripts/run.sh 1 1 # $1=validator Id; $2=operating role
    ./scripts/run.sh 2 1
    ./scripts/run.sh 3 1

## Important parameter explanation

System setup parameters:

  `-b int` is the universal batch size used in both ordering and consensus instances\
  `-boo int` is the size of booths\
  `-c int` is the maximum connections a vehicle can connect to (`c` should always be greater than or equal to `boo`)\
  `-ci int` is the interval for scheduled consensus instances\
  `-m int` is the message size (default 32 bytes)\
  `-s bool` if enabled, logs are be stored into ./log/s_<server_id>
  
Parameters critical for performance tuning:

`-w int` defines the number of worker threads that concurrently initiate ordering instances\
`-gc bool` enables garbage collection if set to true\
`-es int` defines the number of cycles that the proposer intensively produces data entries\
`-ed int` defines the time that the proposer release resources after `es` cycles

## Deployment
The project is under a double-blind review process. 
Our deployment details may disclose our institution information, so they are temporarily redacted to preserve anonymity.
