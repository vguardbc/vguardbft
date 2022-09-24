package main

type ProposeEntry struct {
	Timestamp	int64
	Transaction []byte
}

type QuorumConf struct {
	Continuation bool
	PrevQuorumHash []byte
	Identities map[[32]byte]int //key is pub key hash; value is server id
}

type LeaderOrderingAEntry struct {
	QuorumConf
	BlockId int64
	PartialSignature []byte

	//HashOfBatchedEntries is the common input of the penSign function
	HashOfBatchedEntries []byte
	//Entries map[int]Entry

	//Private:
	hashedForSig []byte
}

type Entry struct {
	TimeStamp 	int64
	Tx			[]byte
}

// Sent to Coordinator 2
type WorkerOrderingAReply struct {
	BlockId int64
	PartialSignature []byte //this is a threshold signature
}

type LeaderOrderingBEntry struct {
	BlockId int64
	CombinedSignatures []byte //this is a combined threshold signature
	Entries map[int]Entry
}

// Sent to Coordinator 3
//type WorkerOrderingBReply struct {
//	BlockId int64
//	PartialSignature []byte
//}

type LeaderEntryCA struct {
	QuorumConf
	BIDStart	int64
	RangeId		int64
	RangeHash 	map[int64][]byte
	TotalHash 	[]byte
	//CombinedSignatures []byte //this is a combined threshold signature
}

type WorkerReplyCA struct {
	RangeId	int64
	ParSig	[]byte
}

type LeaderEntryCB struct {
	QuorumConf
	RangeId int64
	ComSig	[]byte //this is a combined threshold signature
}

type WorkerReplyCB struct {
	RangeId int64
	//ParSig	[]byte
	done bool
}