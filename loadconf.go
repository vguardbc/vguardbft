package main

import (
	"bufio"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func newParse(numOfServers, ThisServerID int) {
	// parseClusterCrypto(ThisServerID)
	parseClusterConf(numOfServers, ThisServerID)
	fmt.Printf(">>>> config files fetched and parsed\n")
}

func parseClusterCrypto(ThisServerID int) {
	var fileRows []string

	c, err := os.Open(fmt.Sprintf("./config/crypto_%d.conf", ThisServerID))

	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(c)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		fileRows = append(fileRows, scanner.Text())
	}

	err = c.Close()
	if err != nil {
		log.Errorf("close crypto.conf failed | err: %v\n", err)
	}

	var prik []byte

	for i := 0; i < len(fileRows); i++ {
		row := strings.Fields(fileRows[i])

		//first line is this server's private key
		if i == 0 {
			prik, err = hex.DecodeString(row[1])
			if err != nil {
				log.Fatalf("Decode private key string failed | err: %v", err)
				return
			}

			err = json.Unmarshal(prik, &PrivateKey)
			if err != nil {
				log.Fatalf("unmarshal private key bytes failed | err: %v", err)
				return
			}
			continue
		}

		pubk, err := hex.DecodeString(row[1])
		if err != nil {
			log.Fatalf("Decode S%d public key string failed | err: %v", i-1, err)
			return
		}

		var publicKey *rsa.PublicKey
		err = json.Unmarshal(pubk, &publicKey)
		if err != nil {
			log.Fatalf("unmarshal S%d public key bytes failed | err: %v", i-1, err)
			return
		}

		PublicKeys = append(PublicKeys, publicKey)
	}

	fmt.Printf("Fetch server crypto keys finished:\n")
	fmt.Printf("Server %d PrivateKey => %s\n", ThisServerID, hex.EncodeToString(prik))
	for i := 0; i < len(PublicKeys); i++ {
		pub, _ := json.Marshal(PublicKeys[i])
		fmt.Printf("Server %d PublicKey -> %s\n", i, hex.EncodeToString(pub))
	}
}

func parseClusterConf(numOfServers, ThisServerID int) {
	var fileRows []string

	s, err := os.Open(ConfigFilePath)
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

		ServerSecrets = append(ServerSecrets, []byte(row[2]))

		singleSL.Ports = make(map[int]string)

		singleSL.Ports[ListenerPortOfOrderingA] = row[3]
		singleSL.Ports[ListenerPortOfOrderingB] = row[4]
		singleSL.Ports[ListenerPortOfConsensusA] = row[5]
		singleSL.Ports[ListenerPortOfConsensusB] = row[6]
		singleSL.Ports[DialPortOfOrderingPhaseA] = row[7]
		singleSL.Ports[DialPortOfOrderingPhaseB] = row[8]
		singleSL.Ports[DialPortOfConsensusPhaseA] = row[9]
		singleSL.Ports[DialPortOfConsensusPhaseB] = row[10]

		serverConnRegistry.Lock()
		serverConnRegistry.m[singleSL.Ip+":"+row[7]] = singleSL.Index
		serverConnRegistry.m[singleSL.Ip+":"+row[8]] = singleSL.Index
		serverConnRegistry.m[singleSL.Ip+":"+row[9]] = singleSL.Index
		serverConnRegistry.m[singleSL.Ip+":"+row[10]] = singleSL.Index
		serverConnRegistry.Unlock()

		log.Infof("fetched server %d config file: %+v\n", singleSL.Index, singleSL)
		ServerList = append(ServerList, singleSL)
	}
}
