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
	"fmt"

	"github.com/spf13/viper"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto"
	sysccapi "github.com/hyperledger/fabric/core/system_chaincode/api"
	pb "github.com/hyperledger/fabric/protos"
)

func Wrap(tx *pb.Transaction) (*pb.Transaction, error) {
	var outertx *pb.Transaction
	var err error

	/***** NEW Approach - send func, args and compute binary on each validator
	newArgs := make([]string, len(cds.ChaincodeSpec.CtorMsg.Args)+1)
	newArgs[0] = cds.ChaincodeSpec.CtorMsg.Function
	for _,arg := range cds.ChaincodeSpec.CtorMsg.Args {
		newArgs = append(newArgs, arg)
	}
	**************/

	//for now old approach .... Base64 encoding... ugh
	var data []byte
	data, err = proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal chaincode deployment spec : %s", err)
	}
	newArgs := make([]string, 1)
	newArgs[0] = base64.StdEncoding.EncodeToString([]byte(data))

	var funcName string
  if tx.Type == pb.Transaction_CHAINCODE_DEPLOY{
    funcName = DEPLOY
  }else if tx.Type == pb.Transaction_CHAINCODE_UPGRADE {
		funcName = UPGRADE
	}

	spec := &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{Name: sysccapi.UBER},
		CtorMsg:     &pb.ChaincodeInput{Function: funcName, Args: newArgs}}

	uuid := tx.Uuid

	if viper.GetBool("security.enabled") {
	   logger.Debug("Initializing secure client using context %s", spec.SecureContext)
		sec, err := crypto.InitClient(spec.SecureContext, nil)
		defer crypto.CloseClient(sec)

		// remove the security context since we are no longer need it down stream
		spec.SecureContext = ""

		if nil != err {
			return nil, err
		}
		logger.Debug("Creating secure transaction %s", uuid)
		outertx, err = sec.NewChaincodeExecute(&pb.ChaincodeInvocationSpec{spec}, uuid)
	} else {
		logger.Debug("Creating deployment transaction (%s)", uuid)
		outertx, err = pb.NewChaincodeExecute(&pb.ChaincodeInvocationSpec{spec}, uuid, pb.Transaction_CHAINCODE_INVOKE)
	}
	return outertx, nil
}
