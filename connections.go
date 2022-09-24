package main

import (
	"net"
)

func runAsLeaderAndAcceptServerConnections(leaderId ServerId) {

	go acceptOrderingPhaseAConnections(leaderId)
	go acceptOrderingPhaseBConnections(leaderId)
	go acceptConsensusPhaseAConnections(leaderId)
	go acceptConsensusPhaseBConnections(leaderId)

	txGenerator(MsgSize)

	// Waiting for connections to be established first
	for i := 0; i < NOP; i++ {
		<-connectedChannel
		log.Debugf("connection completed %v, expected %v", i, NOP)
	}

	for i := 0; i < NumOfWorker; i++ {
		go startOrderingPhaseA(i)
	}

	go startConsensusPhaseA()
}

func closeTCPListener(l *net.Listener, phaseNum Phase) {
	err := (*l).Close()
	if err != nil {
		log.Errorf("close Phase %s TCP listener failed | err: %v", phaseNum, err)
	}
}

func acceptOrderingPhaseAConnections(leaderId ServerId) {
	postPhaseListenerAddress := ServerList[leaderId].Ip + ":" + ServerList[leaderId].Ports[ListenerPortOfOrderingA]
	postPhaseListener, err := net.Listen("tcp4", postPhaseListenerAddress)

	log.Infof("OrderingPhaseA listener is up at %s", postPhaseListener.Addr().String())

	defer closeTCPListener(&postPhaseListener, ListenerPortOfOrderingA)

	if err != nil {
		log.Error(err)
		return
	}

	hasInvoked := 0
	for {
		if conn, err := postPhaseListener.Accept(); err == nil {
			go handleOrderingPhaseAServerConn(&conn)
		} else {
			log.Error(err)
		}

		hasInvoked++
		if hasInvoked == NumOfConn-1 {
			log.Infof("Closing %s Listener: %s; %d servers connected",
				cmdPhase[OrderingPhaseA], postPhaseListener.Addr().String(), hasInvoked)
			break
		}
	}
	connectedChannel <- 1
}

func acceptOrderingPhaseBConnections(leaderId ServerId) {
	orderPhaseListenerAddress := ServerList[leaderId].Ip + ":" + ServerList[leaderId].Ports[ListenerPortOfOrderingB]
	orderPhaseListener, err := net.Listen("tcp4", orderPhaseListenerAddress)

	defer closeTCPListener(&orderPhaseListener, ListenerPortOfOrderingB)

	log.Infof("OrderingPhaseB listener is up at %s", orderPhaseListener.Addr().String())

	if err != nil {
		log.Error(err)
		return
	}

	hasInvoked := 0
	for {
		if conn, err := orderPhaseListener.Accept(); err == nil {
			go handleOrderingPhaseBServerConn(&conn)
		} else {
			log.Error(err)
		}

		hasInvoked++
		if hasInvoked == NumOfConn-1 {
			log.Infof("Closing %s Listener: %s; %d servers connected",
				cmdPhase[OrderingPhaseB], orderPhaseListener.Addr().String(), hasInvoked)
			break
		}
	}
	connectedChannel <- 1
}

func acceptConsensusPhaseAConnections(leaderId ServerId) {
	preConsensusListenerAddress := ServerList[leaderId].Ip + ":" + ServerList[leaderId].Ports[ListenerPortOfConsensusA]
	preConsensusListener, err := net.Listen("tcp4", preConsensusListenerAddress)

	defer closeTCPListener(&preConsensusListener, ListenerPortOfConsensusA)

	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("CommitPhaseA listener is up at %s", preConsensusListener.Addr().String())

	hasInvoked := 0
	for {
		if conn, err := preConsensusListener.Accept(); err == nil {
			go handleCommitPhaseAServerConn(&conn)
		} else {
			log.Error(err)
		}

		hasInvoked++
		if hasInvoked == NumOfConn-1 {
			log.Infof("Closing %s Listener: %s; %d servers connected",
				cmdPhase[CommitPhaseA], preConsensusListener.Addr().String(), hasInvoked)
			break
		}
	}
	connectedChannel <- 1
}

func acceptConsensusPhaseBConnections(leaderId ServerId) {
	CBListenerAddress := ServerList[leaderId].Ip + ":" + ServerList[leaderId].Ports[ListenerPortOfConsensusB]
	CBListener, err := net.Listen("tcp4", CBListenerAddress)

	defer closeTCPListener(&CBListener, ListenerPortOfConsensusB)

	if err != nil {
		log.Error(err)
		return
	}

	log.Infof("CommitPhaseB listener is up at %s", CBListener.Addr().String())

	hasInvoked := 0
	for {
		if conn, err := CBListener.Accept(); err == nil {
			go handleCommitPhaseBServerConn(&conn)
		} else {
			log.Error(err)
		}

		hasInvoked++
		if hasInvoked == NumOfConn-1 {
			log.Infof("Closing %s Listener: %s; %d servers connected",
				cmdPhase[CommitPhaseB], CBListener.Addr().String(), hasInvoked)
			break
		}
	}
	connectedChannel <- 1
}
