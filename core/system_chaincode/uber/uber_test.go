/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package uber

import (
	"testing"
  "encoding/base64"
  "github.com/golang/protobuf/proto"
  pb "github.com/hyperledger/fabric/protos"
  "github.com/hyperledger/fabric/core/container"
  "github.com/hyperledger/fabric/core/chaincode"
  "golang.org/x/net/context"
  "github.com/spf13/viper"
)

func TestUberCCDeploy(t *testing.T) {
  viper.Set("peer.fileSystemPath", "/var/hyperledger/test/uber")
  deployTx := constructDeployTx(t, "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example01", "init", []string{"a", "100", "b", "200"})
  ctxt := context.Background()
  var ccSupport *chaincode.ChaincodeSupport
	//sysccapi.RegisterSysCC(name string, path string, o interface{})
  //ccSupport = chaincode.NewChaincodeSupport(chainname chaincode.ChainName, getPeerEndpoint func()
  t.Logf("ccSupport = %s", ccSupport)
  chaincode.Execute(ctxt, ccSupport, deployTx)
}

func constructDeployTx(t *testing.T, url string, initFuncName string, initArgs []string) *pb.Transaction {
  cds := constructDeploymentSpec(t, url, initFuncName, initArgs)
  txID := cds.ChaincodeSpec.ChaincodeID.Name
  deployTx, err := pb.NewChaincodeDeployTransaction(cds, txID)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	return wrapInUberTx(t, deployTx)
}

func constructDeploymentSpec(t *testing.T, url string, initFuncName string, initArgs []string) *pb.ChaincodeDeploymentSpec {
  spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Path: url}, CtorMsg: &pb.ChaincodeInput{Function: initFuncName, Args: initArgs}}
	codePackageBytes, err := container.GetChaincodePackageBytes(spec)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes, ExecEnv:1}
	return chaincodeDeploymentSpec
}

func encodeBase64(t *testing.T, tx *pb.Transaction) string {
  data, err := proto.Marshal(tx)
  if err != nil {
    t.Fatalf("Error: %s", err)
  }
  return base64.StdEncoding.EncodeToString(data)
}

func wrapInUberTx(t *testing.T, tx *pb.Transaction) *pb.Transaction {
	uberTx, err := Wrap(tx)
	if err != nil {
    t.Fatalf("Error: %s", err)
  }
	return uberTx
}
