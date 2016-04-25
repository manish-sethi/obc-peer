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
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	sysccapi "github.com/hyperledger/fabric/core/system_chaincode/api"
	pb "github.com/hyperledger/fabric/protos"
)

const (
	DEPLOY  = "deploy"
	START   = "start"
	STOP    = "stop"
	UPGRADE = "upgrade"

	GETTRAN = "get-transaction"
)

//errors
type InvalidFunctionErr string

func (f InvalidFunctionErr) Error() string {
	return fmt.Sprintf("invalid function to uber %s", string(f))
}

type InvalidArgsLenErr int

func (i InvalidArgsLenErr) Error() string {
	return fmt.Sprintf("invalid number of argument to uber %d", int(i))
}

type InvalidArgsErr int

func (i InvalidArgsErr) Error() string {
	return fmt.Sprintf("invalid argument (%d) to uber", int(i))
}

type TXExistsErr string

func (t TXExistsErr) Error() string {
	return fmt.Sprintf("transaction exists %s", string(t))
}

type TXNotFoundErr string

func (t TXNotFoundErr) Error() string {
	return fmt.Sprintf("transaction not found %s", string(t))
}

// UberSysCC implements chaincode lifecycle and policies aroud it
type UberSysCC struct {
}

func (t *UberSysCC) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil, nil
}

// Invoke meta transaction to uber with functions "deploy", "start", "stop", "upgrade"
func (t *UberSysCC) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != DEPLOY && function != START && function != STOP && function != UPGRADE {
		return nil, InvalidFunctionErr(function)
	}

	tx, txbytes, err := sysccapi.GetTransaction(args)

	if err != nil {
		return nil, err
	}

	switch tx.Type {
	case pb.Transaction_CHAINCODE_UPGRADE:
		return t.upgradeChaincode(stub, tx, txbytes)
	case pb.Transaction_CHAINCODE_DEPLOY:
		return t.deployChaincode(stub, tx, txbytes)
	}
	return nil, nil
}

func (t *UberSysCC) deployChaincode(stub *shim.ChaincodeStub, tx *pb.Transaction, txBytes []byte) ([]byte, error) {
	cds := &pb.ChaincodeDeploymentSpec{}
	err := proto.Unmarshal(tx.Payload, cds)
	if err != nil {
		return nil, err
	}

	barr, err := stub.GetState(cds.ChaincodeSpec.ChaincodeID.Name)
	if barr != nil {
		return nil, TXExistsErr(cds.ChaincodeSpec.ChaincodeID.Name)
	}

	err = sysccapi.Deploy(tx)

	if err != nil {
		return nil, fmt.Errorf("Error deploying %s: %s", cds.ChaincodeSpec.ChaincodeID.Name, err)
	}

	err = stub.PutState(cds.ChaincodeSpec.ChaincodeID.Name, txBytes)
	if err != nil {
		return nil, fmt.Errorf("PutState failed for %s: %s", cds.ChaincodeSpec.ChaincodeID.Name, err)
	}

	err = stub.PutState(constructStateKeyForChaincodeInfo(cds.ChaincodeSpec.ChaincodeID.Name), []byte("Construct a proto message for storing state|mode|base-chaincode"))
	if err != nil {
		return nil, fmt.Errorf("PutState failed for info of %s: %s", cds.ChaincodeSpec.ChaincodeID.Name, err)
	}

	fmt.Printf("Successfully deployed ;-)\n")

	return nil, err
}

func (t *UberSysCC) upgradeChaincode(stub *shim.ChaincodeStub, tx *pb.Transaction, txBytes []byte) ([]byte, error) {
	cus := &pb.ChaincodeUpgradeSpec{}
	err := proto.Unmarshal(tx.Payload, cus)
	if err != nil {
		return nil, err
	}
	baseCCInfoBytes, err := stub.GetState(constructStateKeyForChaincodeInfo(cus.BaseChaincodeName))
	if baseCCInfoBytes == nil {
		return nil, fmt.Errorf("Base chaincode [%s] does not exist", cus.BaseChaincodeName)
	}
	if err != nil {
		return nil, fmt.Errorf("Error upgrading %s: %s", cus.ChaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name, err)
	}

	// morph the tx into a deploy tx
	chaincodeName := cus.ChaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name
	chaincodeStateKey := constructStateKeyForChaincodeInfo(chaincodeName)
	tx.Type = pb.Transaction_CHAINCODE_DEPLOY
	tx.Payload, err = proto.Marshal(cus.GetChaincodeDeploymentSpec())
	if err != nil {
		return nil, fmt.Errorf("Error upgrading %s: %s", chaincodeName, err)
	}
	txBytes, err = proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("Error upgrading %s: %s", chaincodeName, err)
	}

	t.deployChaincode(stub, tx, txBytes)
	b, err := stub.GetState(chaincodeStateKey)
	if err != nil {
		return nil, fmt.Errorf("Error in getting state during upgrade of %s: %s", chaincodeName, err)
	}
	// add base chaincode name to 'b'
	// b = b + BaseChaincodeName
	err = stub.PutState(chaincodeStateKey, b)
	return nil, err
}

// Query callback representing the query of a chaincode
func (t *UberSysCC) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != GETTRAN {
		return nil, InvalidFunctionErr(function)
	}

	if len(args) != 1 {
		return nil, InvalidArgsLenErr(len(args))
	}

	txbytes, _ := stub.GetState(args[0])
	if txbytes == nil {
		return nil, TXNotFoundErr(args[0])
	}

	return []byte(fmt.Sprintf("Found pb.Transaction for %s", args[0])), nil
}

func constructStateKeyForChaincodeInfo(chaincodeName string) string {
	return fmt.Sprintf("%s_%s", chaincodeName, "info")
}
