package main

import (
	"errors"
	"gonum.org/v1/gonum/stat/combin"
	"io"
	"strconv"
	"strings"
	"sync"
)

type Booth struct {
	ID      int
	Indices []int

	//key is pub key hash; value is server id
	//Identities map[[32]byte]int
}

func (b *Booth) String() (string, error) {
	if len(b.Indices) == 0 {
		return "", errors.New("booth is nil")
	}

	out := make([]string, len(b.Indices))
	for i, v := range b.Indices {
		out[i] = strconv.Itoa(v)
	}

	return strings.Join(out, ""), nil
}

//booMgr is the queue of all enqueued booths
var booMgr = struct {
	sync.RWMutex
	b []Booth
}{}

func prepareBooths(numOfConns, boothsize int) {
	total, boothsize := numOfConns-1, boothsize-1

	if total < boothsize {
		total = boothsize
		log.Warnf("total is %v, which is less than %v; setting total=boothsize", total, boothsize)
	}

	boothsIndices := combin.Combinations(total, boothsize)
	log.Infof("total: %v, boothsize: %v| generated booth indices: %v", total, boothsize, boothsIndices)

	for boothId, memberIds := range boothsIndices {
		log.Debugf("BoothID: %v -> Members: %v", boothId, memberIds)

		// Filtering out booths without memberID 0, as a booth must contain proposer and pivot validator.
		// MemberID 0 is a symbol for the combination of the proposer and pivot validator. Thus, the number of members
		// now is one less than the actual number of members. The next increments member IDs and member 0 back to the
		// booth.

		pivotFlag := false
		for i := 0; i < len(memberIds); i++ {
			if memberIds[i] == 0 {
				pivotFlag = true
			}
			memberIds[i]++
		}

		if !pivotFlag {
			continue
		}

		memberIds = append(memberIds, 0)

		booMgr.b = append(booMgr.b, Booth{
			ID:      boothId,
			Indices: memberIds,
		})

	}
	log.Infof("enqueued booths: %v", booMgr.b)
}

var broadcastError = false

// broadcastToBooth is used by the proposer to broadcast a given message to all members in a given booth
//
func broadcastToBooth(e interface{}, phase int, boothID int) {
	if broadcastError {
		return
	}

	boo := booMgr.b[boothID]

	for _, i := range boo.Indices {
		if ServerID == i {
			continue
		}

		if concierge.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, e)
			continue
		}

		err := concierge.n[phase][i].enc.Encode(e)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}
}

func broadcastToNewBooth(regularMsg interface{}, phase int, boothID int, newMemberIDs []int, newMsg interface{}) {
	if broadcastError {
		return
	}

	boo := booMgr.b[boothID]

	for _, i := range boo.Indices {
		var err error

		if ServerID == i {
			continue
		}

		if concierge.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, regularMsg)
			continue
		}

		newMemberFlag := false
		for _, newMember := range newMemberIDs {
			if newMember == i {
				//log.Errorf("newMember: %v is not in Booth: %v", newMember, boo.Indices)
				newMemberFlag = true
				err = concierge.n[phase][i].enc.Encode(newMsg)
			}
		}

		if newMemberFlag {
			continue
		}

		err = concierge.n[phase][i].enc.Encode(regularMsg)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}
}

// broadcastToAll is used by the proposer to broadcast a given message to all connected members.
//
func broadcastToAll(e interface{}, phase int) {
	if broadcastError {
		return
	}

	for i := 0; i < len(concierge.n[phase]); i++ {
		if ServerID == i {
			continue
		}

		if concierge.n[phase][i] == nil {
			log.Errorf("server %v is not registered in phase %v | msg tried to sent %v:", i, phase, e)
			continue
		}

		err := concierge.n[phase][i].enc.Encode(e)
		if err != nil {
			broadcastError = true
			switch err {
			case io.EOF:
				log.Errorf("server %v closed connection | err: %v", concierge.n[phase][i].SID, err)
				break
			default:
				log.Errorf("sent to server %v failed | err: %v", concierge.n[phase][i].SID, err)
			}
		}
	}
}
