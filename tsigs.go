package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"os"
)

var suite = bn256.NewSuite()

//var ServerSecrets [][]byte
var PublicPoly *share.PubPoly
var PrivateShare *share.PriShare

func fetchKeys(t, id int) {
	//var keys KeyMaster
	//keys.FetchKeys(t, id)
	var err error

	PublicPoly, err = fetchPubPoly(t)
	if err != nil {
		log.Error(err)
		panic(errors.New("fetchPubPoly failed"))
	}

	PrivateShare, err = fetchPriShare(id, t)
	if err != nil {
		log.Error(err)
		panic(errors.New("fetchPriShare failed"))
	}

	return
}

type KeyMaster struct {
	PubPoly  *share.PubPoly
	PriShare *share.PriShare
	path     string
}

func (k *KeyMaster) FetchKeys(t, id int) {
	pub, err := fetchPubPoly(t)
	if err != nil {
		log.Error(err)
		panic(errors.New("fetchPubPoly failed"))
	}
	k.PubPoly = pub

	pris, err := fetchPriShare(id, t)
	if err != nil {
		log.Error(err)
		panic(errors.New("fetchPriShare failed"))
	}
	k.PriShare = pris
}

func fetchPubPoly(t int) (*share.PubPoly, error) {
	readPubPoly, err := os.Open("./keys/vguard_pub.dupe")

	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(readPubPoly)
	scanner.Split(bufio.ScanLines)
	var txtlines []string

	for scanner.Scan() {
		txtlines = append(txtlines, scanner.Text())
	}

	if err := readPubPoly.Close(); err != nil {
		return nil, err
	}

	if len(txtlines) != t {
		return nil, errors.New(fmt.Sprintf("txtlen: %v | threshold: %v \n", len(txtlines), t))
	}

	commits := make([]kyber.Point, t)

	suite := bn256.NewSuite()
	for i, line := range txtlines {
		b, err := hex.DecodeString(line)
		if err != nil {
			return nil, err
		}

		s := suite.G2().Scalar()

		err = s.UnmarshalBinary(b)
		if err != nil {
			return nil, err
		}

		commits[i] = suite.G2().Point().Mul(s, suite.G2().Point().Base())
	}

	return share.NewPubPoly(suite.G2(), suite.G2().Point().Base(), commits), nil
}

func fetchPriShare(serverId int, t int) (*share.PriShare, error) {
	suite := bn256.NewSuite()
	rand := suite.RandomStream()
	secret := suite.G1().Scalar().Pick(rand)
	priPoly := share.NewPriPoly(suite.G2(), t, secret, rand)

	priShare := priPoly.Shares(1)[0]

	readPriShare, err := os.Open(fmt.Sprintf("./keys/pri_%d.dupe", serverId))
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(readPriShare)
	scanner.Split(bufio.ScanLines)
	var txtlines []string

	for scanner.Scan() {
		txtlines = append(txtlines, scanner.Text())
	}

	if err := readPriShare.Close(); err != nil {
		return nil, err
	}

	if len(txtlines) != 1 {
		return nil, errors.New("pri share more than one line")
	}

	priShare.I = serverId
	bytesShare, err := hex.DecodeString(txtlines[0])
	if err != nil {
		return nil, err
	}

	err = priShare.V.UnmarshalBinary(bytesShare)
	if err != nil {
		return nil, err
	}

	return priShare, nil
}

func getDigest(x []byte) []byte {
	r := sha256.Sum256(x)
	return r[:]
}

//func signMsgDigest(tx string) ([]byte, []byte, error) {
//	digest := getDigest([]byte(tx))
//	sig, err := PenSign(digest[:])
//	return digest[:], sig, err
//}

func PenSign(msg []byte) ([]byte, error) {
	return tbls.Sign(suite, PrivateShare, msg)
}

func PenVerifyPartially(msg, sig []byte, pub *share.PubPoly) error {
	return tbls.Verify(suite, pub, msg, sig)
}

//func PenVerifyPartially(msg, sig []byte) (int, error) {
//	return penKeyGen.Verify(suite, PublicPoly, msg, sig)
//}

func PenRecovery(sigShares [][]byte, msg *[]byte, pub *share.PubPoly) ([]byte, error) {
	sig, err := tbls.Recover(suite, pub, *msg, sigShares, Threshold, BoothSize)
	return sig, err
}

func PenVerify(msg, sig []byte, pub *share.PubPoly) error {
	return bls.Verify(suite, pub.Commit(), msg, sig)
}
