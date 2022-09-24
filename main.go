package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"sync"
)

var log = logrus.New()
var metre latencyMetre

func init() {
	loadCmdParameters()
	setLogger()
	newParse(NumOfConn, ThisServerID)
	initServerConnNavigatorAndDialogConnRegistryConns(NumOfConn)
	fetchKeyGen()
	metre.init()

	fmt.Printf("-------------------------------\n")
	fmt.Printf("|- System  information board -|\n")
	fmt.Printf("|-----------------------------|\n")
	fmt.Printf("| Batch size\t| %3d\t|\n", BatchSize)
	fmt.Printf("| Server ID\t| %3d\t|\n", ThisServerID)
	fmt.Printf("| Log level\t| %3d\t|\n", LogLevel)
	fmt.Printf("| # of servers\t| %3d\t|\n", NumOfConn)
	fmt.Printf("| Init role\t| %3d\t|\n", Role)
	fmt.Printf("| Quorum size\t| %3d\t|\n", Quorum)
	//fmt.Printf("| reqQueue\t| %3d\t|\n", cap(requestQueue))
	fmt.Printf("-------------------------------\n")
}

func main() {

	log.Infof("Loading files completed; now program starts")
	runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup
	wg.Add(1)

	go start()

	wg.Wait()
}

func setLogger() {
	if PlainStorage {
		e := os.RemoveAll(fmt.Sprintf("./logs/s%d", ThisServerID))
		if e != nil {
			log.Fatal(e)
		}
		fmt.Println(">> old logs removed at " + fmt.Sprintf("./logs/s%d", ThisServerID))

		if err := os.Mkdir(fmt.Sprintf("./logs/s%d", ThisServerID), os.ModePerm); err != nil {
			log.Error(err)
		}
		fmt.Println(">>> new log folder created at " + fmt.Sprintf("./logs/s%d", ThisServerID))

		// runtimeOfLogrus "github.com/banzaicloud/logrus-runtime-formatter"
		// runtimeFormatter := &runtimeOfLogrus.Formatter{
		//	ChildFormatter: &logrus.TextFormatter{},
		//	Line:           true,
		//	File:           true,
		//	BaseNameOnly: 	true,
		//}
		//log.Formatter = runtimeFormatter
		log.Formatter = &logrus.TextFormatter{
			ForceColors:               false,
			DisableColors:             true,
			ForceQuote:                false,
			EnvironmentOverrideColors: false,
			DisableTimestamp:          true,
			FullTimestamp:             false,
			TimestampFormat:           "",
			DisableSorting:            false,
			SortingFunc:               nil,
			DisableLevelTruncation:    false,
			PadLevelText:              false,
			QuoteEmptyFields:          false,
			FieldMap:                  nil,
			CallerPrettyfier:          nil,
		}

		log.Out = os.Stdout
		fileName := fmt.Sprintf("./logs/s%d/n_%d_b%d_d%d.log", ThisServerID, NumOfConn, BatchSize, Delay)
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.Out = file
		} else {
			log.Info("Failed to log to file, using default stderr")
		}
	}

	log.SetLevel(logrus.Level(LogLevel))
	log.Info("Logger initialization completed.")
}
