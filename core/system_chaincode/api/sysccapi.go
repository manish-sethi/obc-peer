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

package api

import (
	"encoding/base64"
	"fmt"
	"github.com/op/go-logging"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"golang.org/x/net/context"
	inproc "github.com/hyperledger/fabric/core/container/inproccontroller"
	pb "github.com/hyperledger/fabric/protos"
)

const (
	UBER = "uber"
)

var sysccLogger = logging.MustGetLogger("sysccapi")

var registeredSystemCCs = make(map[string]string)

var uberUp bool

// RegisterSysCC registers a system chaincode with the given path
func RegisterSysCC(name string, path string, o interface{}) error {
	syscc := o.(shim.Chaincode)
	if syscc == nil {
		sysccLogger.Warning(fmt.Sprintf("invalid chaincode %v", o))
		return fmt.Errorf(fmt.Sprintf("invalid chaincode %v", o))
	}
	if _,ok := registeredSystemCCs[name]; ok {
		return fmt.Errorf(fmt.Sprintf("syscc %s already registered", name))
	}
	err := inproc.Register(path, syscc)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("could not register (%s,%v): %s", path, syscc, err))
	}
	registeredSystemCCs[name] = path
	sysccLogger.Debug("system chaincode %s registered", path)
	return err
}

func IsSystemCC(name string) bool {
	if _,ok := registeredSystemCCs[name]; ok {
		return true
	}
	return false
}

//GetTransaction returns a transaction given args
//First implementation unmarshals base64 encoded string
func GetTransaction(args []string) (*pb.Transaction, []byte, error) {
	/***** NEW Approach construct image from ChaincodeID, func and args
	var depFunc string
	var depArgs []string
	if len(args) > 0 {
		depFunc = args[0]
		depArgs = args[1:]
	}
	fmt.Printf("Uber deploying %s, %v\n", depFunc, depArgs)
	**************/

	if len(args) != 1  {
		return nil, nil, fmt.Errorf("Invalid number of arguments %d", len(args))
	}

	//for now use old approach
	data, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		return nil, nil, fmt.Errorf("Error decoding chaincode deployment spec %s", err)
	}


	t := &pb.Transaction{}

	//TODO: need to use crypto decrypt t when using security
	err = proto.Unmarshal(data, t)
	if err != nil {
		err = fmt.Errorf("Transaction unmarshal failed: %s", err)
		sysccLogger.Error(fmt.Sprintf("%s", err))
		return nil, nil, err
	}

	return t, data, nil
}

func Deploy(t *pb.Transaction) error {
	chaincodeSupport := chaincode.GetChain(chaincode.DefaultChain)
	ctxt := context.Background()
	_, err := chaincodeSupport.DeployChaincode(ctxt, t)
	if err != nil {
		return fmt.Errorf("Failed to deploy chaincode spec(%s)", err)
	}
	//launch and wait for ready
	_, _, err = chaincodeSupport.LaunchChaincode(ctxt, t)
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	return nil
}
