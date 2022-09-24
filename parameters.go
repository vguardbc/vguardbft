package main

import (
	"crypto/rsa"
	"flag"
	"github.com/dedis/kyber/pairing/bn256"
	"github.com/dedis/kyber/share"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

const NOP = 4                   // number of phases
const MaxQueueLength = 10000000 //10,000,000

const (
	OrderingPhaseA = iota
	OrderingPhaseB
	CommitPhaseA
	CommitPhaseB
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
var cmdPhase = []string{"OrderingPhaseA", "OrderingPhaseB", "CommitPhaseA", "CommitPhaseB"}
var replyPhase = []string{"Reply-OrderingPhaseA", "Reply-OrderingPhaseB", "Reply-CommitPhaseA", "Reply-CommitPhaseB"}

var connectedChannel = make(chan int, NOP)

type ServerId int
type Phase int

const (
	ListenerPortOfOrderingA = iota
	ListenerPortOfOrderingB
	ListenerPortOfConsensusA
	ListenerPortOfConsensusB
	//
	DialPortOfOrderingPhaseA
	DialPortOfOrderingPhaseB
	DialPortOfConsensusPhaseA
	DialPortOfConsensusPhaseB
)

var serverConnRegistry = struct {
	sync.RWMutex
	m map[string]ServerId
}{m: make(map[string]ServerId)}

type ServerInfo struct {
	sync.RWMutex
	Index ServerId
	Ip    string
	Ports map[int]string
}

var slowModeFlag = true

var ServerList []ServerInfo

//threshold signatures
var ServerSecrets [][]byte
var PublicPoly *share.PubPoly
var PrivatePoly *share.PriPoly

var PrivateShare *share.PriShare
var suite = bn256.NewSuite()

var Quorum int

//RSA signatures
var PrivateKey *rsa.PrivateKey
var PublicKeys []*rsa.PublicKey

var coordinatorIdOfPhases = struct {
	sync.RWMutex
	lookup map[Phase]ServerId
}{lookup: make(map[Phase]ServerId)}

//below parameters initialized through func loadParametersFromCommandLine()
var (
	BatchSize   int
	MsgSize     int
	MsgLoad     int64
	NumOfWorker int

	NumOfConn int
	BoothSize int
	Threshold int

	LogLevel     int
	ThisServerID int
	Delay        int
	GC           bool
	Role         int
	StopFlag     int

	PeakPerf     bool
	PlainStorage bool

	LatencySampleInterval int64

	CycleEaseSending int
	EasingDuration   int

	ConsensusInterval int

	ConfigFilePath string

	// Only for catering factor evaluation; will delete them later.
	SlowModeCycleNum    int
	SleepTimeInSlowMode int
)

func loadCmdParameters() {
	flag.IntVar(&BatchSize, "b", 1, "batch size")
	flag.IntVar(&MsgSize, "m", 32, "message size")
	flag.Int64Var(&MsgLoad, "ml", 1000000, "# of msg go sent < "+strconv.Itoa(MaxQueueLength)) //1,000,000

	flag.IntVar(&NumOfWorker, "w", 1, "number of worker threads")

	flag.IntVar(&NumOfConn, "c", 4, "max # of connections")
	flag.IntVar(&BoothSize, "boo", 4, "# of vehicles in a booth")

	flag.IntVar(&ThisServerID, "id", 0, "serverID")
	flag.IntVar(&Delay, "d", 0, "network delay")

	flag.IntVar(&StopFlag, "sf", 50, "stop flag waiting for x times")

	flag.BoolVar(&PlainStorage, "s", false, "naive storage")
	flag.BoolVar(&GC, "gc", false, "garbage collection")

	flag.IntVar(&Role, "r", PROPOSER, "0 : Proposer | 1 : Validator")
	flag.BoolVar(&PeakPerf, "perf", true, "peak performance")
	flag.Int64Var(&LatencySampleInterval, "lm", 10, "latency measurement sample interval")

	flag.IntVar(&CycleEaseSending, "es", 0, "# cycles for one easing sending")
	flag.IntVar(&EasingDuration, "ed", 1, "each easing duration (ms)")

	flag.IntVar(&ConsensusInterval, "ci", 100, "consensus instance interval (ms)")

	flag.IntVar(&LogLevel, "log", InfoLevel,
		"0: PanicLevel |"+
			" 1: FatalLevel |"+
			" 2: ErrorLevel |"+
			" 3: WarnLevel |"+
			" 4: InfoLevel |"+
			" 5: DebugLevel")

	flag.IntVar(&SlowModeCycleNum, "sm", 3, "# of cycles going in slow mode")
	flag.IntVar(&SleepTimeInSlowMode, "smt", 1, "slow mode cycle sleep time (second)")

	flag.StringVar(&ConfigFilePath, "cfp", "./config/cluster_localhost.conf", "config file path")
	flag.Parse()

	Quorum = (BoothSize/3)*2 + 1
	Threshold = Quorum - 1 //leader does not self-talking
}

const (
	PanicLevel = iota //0
	FatalLevel        //1
	ErrorLevel        //2
	WarnLevel         //3
	InfoLevel         //4
	DebugLevel        //5
	TraceLevel        //6
)
