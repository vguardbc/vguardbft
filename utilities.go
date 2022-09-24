package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"github.com/dedis/kyber/share"
	"github.com/dedis/kyber/sign/bls"
	kyberOldScalar "go.dedis.ch/kyber"
	"time"
	"vguard/penKeyGen"
)

func serialization(m interface{}) ([]byte, error) {
	return json.Marshal(m)
}

func deserialization(b []byte, m interface{}) error {
	return json.Unmarshal(b, m)
}

func fetchKeyGen() {
	PublicPoly, PrivateShare = FetchPublicPolyAndPrivateKeyShare()
}

func FetchPublicPolyAndPrivateKeyShare() (*share.PubPoly, *share.PriShare) {

	var prosecutorSecrets []kyberOldScalar.Scalar

	for i := 0; i < Threshold; i++ {
		mySecret := ServerSecrets[i]
		secret := suite.G1().Scalar().SetBytes(mySecret)
		prosecutorSecrets = append(prosecutorSecrets, secret)
	}

	PriPoly := share.NewProsecutorPriPoly(suite.G2(), Threshold, prosecutorSecrets)
	PrivatePoly = PriPoly
	return PriPoly.Commit(suite.G2().Point().Base()), PriPoly.Shares(BoothSize)[ThisServerID]
}

func getDigest(x []byte) []byte {
	r := sha256.Sum256(x)
	return r[:]
}

func signMsgDigest(tx string) ([]byte, []byte, error) {
	digest := getDigest([]byte(tx))
	sig, err := PenSign(digest[:])
	return digest[:], sig, err
}

func PenSign(msg []byte) ([]byte, error) {
	return penKeyGen.Sign(suite, PrivateShare, msg)
}

func PenVerifyPartially(msg, sig []byte) (int, error) {
	return penKeyGen.Verify(suite, PublicPoly, msg, sig)
}

func PenRecovery(sigShares [][]byte, msg *[]byte) ([]byte, error) {
	sig, err := penKeyGen.Recover(suite, PublicPoly, *msg, sigShares, Threshold, BoothSize)
	return sig, err
}

func PenVerify(msg, sig []byte) error {
	return bls.Verify(suite, PublicPoly.Commit(), msg, sig)
}

func getHashOfMsg(msg interface{}) ([]byte, error) {
	serializedMsg, err := json.Marshal(msg)
	if err != nil {
		log.Errorf("json marshal failed | err: %v", err)
		return nil, err
	}

	msgHash := sha256.New()
	_, err = msgHash.Write(serializedMsg)
	if err != nil {
		return nil, err
	}

	return msgHash.Sum(nil), nil
}

func cryptoSignMsg(msg interface{}) ([]byte, error) {

	serializedMsg, err := json.Marshal(msg)
	if err != nil {
		log.Errorf("json marshal failed | err: %v", err)
		return nil, err
	}

	msgHash := sha256.New()
	_, err = msgHash.Write(serializedMsg)
	if err != nil {
		return nil, err
	}
	msgHashSum := msgHash.Sum(nil)

	// In order to generate the signature, we provide a random number generator,
	// our private key, the hashing algorithm that we used, and the hash sum
	// of our message
	signature, err := rsa.SignPSS(rand.Reader, PrivateKey, crypto.SHA256, msgHashSum, nil)

	return signature, err
}

func cryptoVerify(publicKey *rsa.PublicKey, msgHashSum, signature []byte) error {
	return rsa.VerifyPSS(publicKey, crypto.SHA256, msgHashSum, signature, nil)
}

func mockRandomBytes(length int, charset string) []byte {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return b
}

// txGenerator enqueues mock data entries to all message queues
//
func txGenerator(len int) {
	log.Debug("INIT >> Loading requests started")

	for i := 0; i < NumOfWorker; i++ {
		q := make(chan *ProposeEntry, MaxQueueLength)

		for i := int64(0); i < MsgLoad; i++ {
			q <- &ProposeEntry{
				Timestamp:   time.Now().UnixMicro(),
				Transaction: mockRandomBytes(len, charset),
			}
		}
		requestQueue = append(requestQueue, q)
	}

	log.Infof("INIT >> %d request queue(s) loaded with %d requests of size %d bytes", NumOfWorker, MsgLoad, MsgSize)
}
