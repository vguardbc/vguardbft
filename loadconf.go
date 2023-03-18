package main

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

func parseConf(numOfServers int) {
	var fileRows []string

	s, err := os.Open(ConfPath)
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(s)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		fileRows = append(fileRows, scanner.Text())
	}

	err = s.Close()
	if err != nil {
		log.Errorf("close fileServer failed | err: %v\n", err)
	}

	//first line is explanation
	if len(fileRows) != numOfServers+1 {
		log.Errorf("Going to panic | fileRows: %v | n: %v", len(fileRows), numOfServers)
		panic(errors.New("number of servers in config file does not match with provided $n$"))
	}

	for i := 0; i < len(fileRows); i++ {
		// Fist line is instructions
		if i == 0 {
			continue
		}

		var singleSL ServerInfo

		row := strings.Split(fileRows[i], " ")

		i, err := strconv.Atoi(row[0])
		if err != nil {
			panic(err)
		}

		singleSL.Index = ServerId(i)

		singleSL.Ip = row[1]

		//ServerSecrets = append(ServerSecrets, []byte(row[2]))

		singleSL.Ports = make(map[int]string)

		singleSL.Ports[ListenerPortOPA] = row[3]
		singleSL.Ports[ListenerPortOPB] = row[4]
		singleSL.Ports[ListenerPortOCA] = row[5]
		singleSL.Ports[ListenerPortOCB] = row[6]
		singleSL.Ports[DialPortOPA] = row[7]
		singleSL.Ports[DialPortOPB] = row[8]
		singleSL.Ports[DialPortCPA] = row[9]
		singleSL.Ports[DialPortCPB] = row[10]

		serverIdLookup.Lock()
		serverIdLookup.m[singleSL.Ip+":"+row[7]] = singleSL.Index
		serverIdLookup.m[singleSL.Ip+":"+row[8]] = singleSL.Index
		serverIdLookup.m[singleSL.Ip+":"+row[9]] = singleSL.Index
		serverIdLookup.m[singleSL.Ip+":"+row[10]] = singleSL.Index
		serverIdLookup.Unlock()

		ServerList = append(ServerList, singleSL)
		log.Debugf("Config file fetched | S%d -> %v:%v \n", singleSL.Index, singleSL.Ip, singleSL.Ports)
	}
}
