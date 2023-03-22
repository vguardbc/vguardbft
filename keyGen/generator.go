package main

/*
Copyright (c) 2022 Gengrui (Edward) Zhang <gengrui.edward.zhang@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
	"os"
	"path"
	"runtime"
	"strconv"
)

func main() {
	var log = logrus.New()
	log.SetReportCaller(true)
	log.SetFormatter(&logrus.TextFormatter{
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			fileName := path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
			//return frame.Function, fileName
			return "", fileName + " >>"
		},
	})

	var (
		t int //threshold
		n int //number of participants
	)

	flag.IntVar(&t, "t", 2, "Threshold")
	flag.IntVar(&n, "n", 6, "# of participants")
	flag.Parse()

	log.Infof("KeyGen initialized, with t=%v and n=%v", t, n)

	suite := bn256.NewSuite()

	rand := suite.RandomStream()
	secret := suite.G1().Scalar().Pick(rand)
	priPoly := share.NewPriPoly(suite.G2(), t, secret, rand)

	if err := os.RemoveAll(fmt.Sprintf("./keys/")); err != nil {
		panic(err)
	}

	if err := os.Mkdir(fmt.Sprintf("./keys/"), os.ModePerm); err != nil {
		panic(err)
	}

	err := createPubPoly(priPoly)
	if err != nil {
		panic(err)
	}
	log.Infof("New PubPoly created at ./keys/vguard_pub.dupe")

	_, err = createPrivateShare(priPoly, n)
	if err != nil {
		panic(err)
	}
	log.Infof("New %v PriShares created in ./keys/", n)
}

func createPubPoly(priPoly *share.PriPoly) error {
	commits := make([]kyber.Point, priPoly.Threshold())
	var commitBytes [][]byte

	for i := range commits {
		binaryScalar, err := priPoly.Coefficients()[i].MarshalBinary()
		if err != nil {
			return err
		}
		commitBytes = append(commitBytes, binaryScalar)
	}

	pubPolyFile, err := os.OpenFile("./keys/vguard_pub.dupe", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	datawriter := bufio.NewWriter(pubPolyFile)

	for _, data := range commitBytes {
		_, err = datawriter.WriteString(hex.EncodeToString(data) + "\n")
		if err != nil {
			return err
		}
	}

	if err := datawriter.Flush(); err != nil {
		return err
	}

	if err := pubPolyFile.Close(); err != nil {
		return err
	}

	return nil
}

func createPrivateShare(priPoly *share.PriPoly, n int) ([]*share.PriShare, error) {
	priShares := priPoly.Shares(n)

	for _, priShare := range priShares {
		priPolyShareFile, err := os.OpenFile(fmt.Sprintf("./keys/pri_%d.dupe", priShare.I), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		datawriter := bufio.NewWriter(priPolyShareFile)

		outBytes, err := priShare.V.MarshalBinary()
		if err != nil {
			return nil, err
		}

		if _, err := datawriter.WriteString(hex.EncodeToString(outBytes) + "\n"); err != nil {
			return nil, err
		}

		if err := datawriter.Flush(); err != nil {
			return nil, err
		}

		if err := priPolyShareFile.Close(); err != nil {
			return nil, err
		}
	}

	return priShares, nil
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
