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
	"encoding/base64"
  "bytes"
  "fmt"
	"net"
	"testing"
	"time"
  "runtime"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/ledger/genesis"
	sysccapi "github.com/hyperledger/fabric/core/system_chaincode/api"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

type uberTestHelper struct{
  t *testing.T
  listener net.Listener
  ledger *ledger.Ledger
}

func newUberTestHelper(t *testing.T) *uberTestHelper{
  h := &uberTestHelper{}
  h.t = t
  h.ledger = ledger.InitTestLedger(t)
  return h
}

func (h *uberTestHelper) deployViaUber(url string, initFuncName string, initArgs []string) string {
  deployTx, ccName := h.constructDeployTxViaUber(url, initFuncName, initArgs)
  h.ledger.BeginTxBatch("1")
  chaincode.Execute(context.Background(), chaincode.GetChain(chaincode.DefaultChain), deployTx)
  h.ledger.CommitTxBatch("1", []*pb.Transaction{deployTx}, nil, nil)
  return ccName
}

func (h *uberTestHelper) upgradeViaUber(parentCCName string, url string, initFuncName string, initArgs []string) string{
  updateTx, newCCName := h.constructUpdateTxViaUber(parentCCName, url, initFuncName, initArgs)
  h.ledger.BeginTxBatch("1")
  chaincode.Execute(context.Background(), chaincode.GetChain(chaincode.DefaultChain), updateTx)
  h.ledger.CommitTxBatch("1", []*pb.Transaction{updateTx}, nil, nil)
  return newCCName
}

func (h *uberTestHelper) executeInvoke(ccName string, funcName string, args []string) {
  invokeTx := h.constructInvokeTx(ccName, funcName, args)
  h.ledger.BeginTxBatch("1")
  chaincode.Execute(context.Background(), chaincode.GetChain(chaincode.DefaultChain), invokeTx)
  h.ledger.CommitTxBatch("1", []*pb.Transaction{invokeTx}, nil, nil)
}

func (h *uberTestHelper) assertState(ccName string, key string, value []byte) {
  v, err := h.ledger.GetState(ccName, key, true)
  if err != nil{
    h.t.Fatalf("Error: %s", err)
  }
  if ! bytes.Equal(v, value) {
    h.t.Fatalf("State in ledger for {chaincode=%s, key=%s}: expected=[%s], found=[%s]. %s", ccName, key, string(value), string(v), getCallerInfo())
  }
}

func (h * uberTestHelper) constructDeployTxViaUber(url string, initFuncName string, initArgs []string) (*pb.Transaction, string) {
	cds := h.constructDeploymentSpec(url, initFuncName, initArgs)
	txID := cds.ChaincodeSpec.ChaincodeID.Name
	deployTx, err := pb.NewChaincodeDeployTransaction(cds, txID)
	if err != nil {
		h.t.Fatalf("Error: %s", err)
	}
	return h.wrapInUberTx(deployTx), cds.ChaincodeSpec.ChaincodeID.Name
}

func (h * uberTestHelper) constructUpdateTxViaUber(parentCCName string, url string, initFuncName string, initArgs []string) (*pb.Transaction, string) {
	cds := h.constructDeploymentSpec(url, initFuncName, initArgs)
	txID := cds.ChaincodeSpec.ChaincodeID.Name
	cds.ChaincodeSpec.ChaincodeID.Parent = parentCCName
	upgradeTx, err := pb.NewChaincodeUpgradeTransaction(cds, txID)
	if err != nil {
		h.t.Fatalf("Error: %s", err)
	}
	return h.wrapInUberTx(upgradeTx), cds.ChaincodeSpec.ChaincodeID.Name
}

func (h * uberTestHelper) constructInvokeTx(cID string, funcName string, args []string) *pb.Transaction {
	ccSpec := &pb.ChaincodeSpec{Type: 1,
		ChaincodeID: &pb.ChaincodeID{Name: cID},
		CtorMsg:     &pb.ChaincodeInput{Function: funcName, Args: args}}
	spec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: ccSpec}
	uuid := util.GenerateUUID()
	tx, err := pb.NewChaincodeExecute(spec, uuid, pb.Transaction_CHAINCODE_INVOKE)
	if err != nil {
		h.t.Fatalf("Error: %s", err)
	}
	return tx
}

func (h * uberTestHelper) constructDeploymentSpec(url string, initFuncName string, initArgs []string) *pb.ChaincodeDeploymentSpec {
	spec := &pb.ChaincodeSpec{Type: 1, ChaincodeID: &pb.ChaincodeID{Path: url}, CtorMsg: &pb.ChaincodeInput{Function: initFuncName, Args: initArgs}}
	codePackageBytes, err := container.GetChaincodePackageBytes(spec)
	if err != nil {
		h.t.Fatalf("Error: %s", err)
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec
}

func (h * uberTestHelper) encodeBase64(tx *pb.Transaction) string {
	data, err := proto.Marshal(tx)
	if err != nil {
		h.t.Fatalf("Error: %s", err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func (h * uberTestHelper) wrapInUberTx(tx *pb.Transaction) *pb.Transaction {
	uberTx, err := Wrap(tx)
	if err != nil {
		h.t.Fatalf("Error: %s", err)
	}
	return uberTx
}


func (h *uberTestHelper) startChaincodeRuntimeWithGRPC() {
	//use a different address than what we usually use for "peer"
	//we override the peerAddress set in chaincode_support.go
	peerAddress := "0.0.0.0:60606"

	lis, err := net.Listen("tcp", peerAddress)
  h.listener = lis

	if err != nil {
		h.t.Fail()
		h.t.Logf("Error starting peer listener %s", err)
		return
	}

	var opts []grpc.ServerOption
	if viper.GetBool("peer.tls.enabled") {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("peer.tls.cert.file"), viper.GetString("peer.tls.key.file"))
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)

	getPeerEndpoint := func() (*pb.PeerEndpoint, error) {
		return &pb.PeerEndpoint{ID: &pb.PeerID{Name: "testpeer"}, Address: peerAddress}, nil
	}

	ccStartupTimeout := time.Duration(30000) * time.Millisecond
	pb.RegisterChaincodeSupportServer(grpcServer, chaincode.NewChaincodeSupport(chaincode.DefaultChain, getPeerEndpoint, false, ccStartupTimeout, nil))

	go grpcServer.Serve(lis)

	sysccapi.RegisterSysCC("uber", "github.com/hyperledger/fabric/core/system_chaincode/uber", &UberSysCC{})
	uberCCSpec := &pb.ChaincodeSpec{Type: 1,
		ChaincodeID: &pb.ChaincodeID{Name: "uber", Path: "github.com/hyperledger/fabric/core/system_chaincode/uber"},
		CtorMsg:     &pb.ChaincodeInput{Function: "", Args: []string{}}}

	ctx := context.Background()
	_, _, err = genesis.DeployLocal(ctx, uberCCSpec, false)
	if err != nil {
		h.t.Fatalf("Error:%s", err)
	}
	h.t.Logf("Chaincode runtime started")
}

func (h *uberTestHelper) stopGRPCServer() {
  if h.listener != nil{
    h.listener.Close()
  }
}

func getCallerInfo() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "Could not retrieve caller's info"
	}
	return fmt.Sprintf("codeLocation = [%s:%d]", file, line)
}
