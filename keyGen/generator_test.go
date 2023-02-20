package main

import (
	"errors"
	"fmt"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"os"
	"testing"
)

func TestPubPoly(t *testing.T) {
	th := 4

	suite := bn256.NewSuite()
	rand := suite.RandomStream()
	secret := suite.G1().Scalar().Pick(rand)
	priPoly := share.NewPriPoly(suite.G2(), th, secret, rand)
	original_pubPoly := priPoly.Commit(suite.G2().Point().Base())

	if err := os.RemoveAll(fmt.Sprintf("./keys/")); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(fmt.Sprintf("./keys/"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	err := createPubPoly(priPoly)
	if err != nil {
		t.Fatal(err)
	}

	pubPoly, err := fetchPubPoly(priPoly.Threshold())

	if !pubPoly.Equal(original_pubPoly) {
		t.Fatal("pub share does not match pubpoly")
	}
}

func TestPriPoly(t *testing.T) {
	th := 4
	n := 10

	suite := bn256.NewSuite()
	rand := suite.RandomStream()
	secret := suite.G1().Scalar().Pick(rand)
	priPoly := share.NewPriPoly(suite.G2(), th, secret, rand)
	original_pubPoly := priPoly.Commit(suite.G2().Point().Base())

	if err := os.RemoveAll(fmt.Sprintf("./keys/")); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(fmt.Sprintf("./keys/"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	err := createPubPoly(priPoly)
	if err != nil {
		t.Fatal(err)
	}
	pubPoly, err := fetchPubPoly(priPoly.Threshold())

	if !pubPoly.Equal(original_pubPoly) {
		t.Fatal("pub share does not match pubpoly")
	}

	// private poly
	orginal_prishares, err := createPrivateShare(priPoly, n)
	if err != nil {
		t.Fatal(err)
	}

	priShares := make([]*share.PriShare, n)

	for i := 0; i < len(priShares); i++ {
		priShares[i], err = fetchPriShare(i, priPoly.Threshold())
		if err != nil {
			t.Fatal(err)
		}
	}

	for i, s := range orginal_prishares {
		if s.String() != priShares[i].String() {
			//t.Logf("recovered %v, %v", i, priShares[i])
			//t.Logf("original %v, %v", i, s)
			t.Fatal(errors.New("pri shares do not match"))
		}
	}
}

func TestThresholdSigs(t *testing.T) {
	th := 4
	n := 10

	suite := bn256.NewSuite()
	rand := suite.RandomStream()
	secret := suite.G1().Scalar().Pick(rand)
	priPoly := share.NewPriPoly(suite.G2(), th, secret, rand)

	if err := os.RemoveAll(fmt.Sprintf("./keys/")); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(fmt.Sprintf("./keys/"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	err := createPubPoly(priPoly)
	if err != nil {
		t.Fatal(err)
	}
	pubPoly, err := fetchPubPoly(priPoly.Threshold())

	// private poly
	_, err = createPrivateShare(priPoly, n)
	if err != nil {
		t.Fatal(err)
	}

	priShares := make([]*share.PriShare, n)

	for i := 0; i < len(priShares); i++ {
		priShares[i], err = fetchPriShare(i, priPoly.Threshold())
		if err != nil {
			t.Fatal(err)
		}
	}

	msg := []byte("vguard is awesome!")
	sigShares := make([][]byte, 0)

	for _, x := range priShares {
		sig, err := tbls.Sign(suite, x, msg)
		if err != nil {
			t.Fatalf("sign err: %v| %v \n", err, x.I)
		}
		sigShares = append(sigShares, sig)

		err = tbls.Verify(suite, pubPoly, msg, sig)
		if err != nil {
			t.Fatalf("verify err: %v| %v \n", err, x.I)
		}
	}

	sig, err := tbls.Recover(suite, pubPoly, msg, sigShares, 4, 10)
	if err != nil {
		t.Fatal(err)
	}

	err = bls.Verify(suite, pubPoly.Commit(), msg, sig)
	if err != nil {
		t.Fatal(err)
	}
}
