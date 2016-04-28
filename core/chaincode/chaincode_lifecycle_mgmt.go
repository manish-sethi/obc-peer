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

package chaincode

import (
	"fmt"

	//"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/core/ledger"
	pb "github.com/hyperledger/fabric/protos"
)

const(
	chaincodeInfoNameSpace = "UBER"
)

type ChaincodeLifecycleManager interface {
  Init(lgr *ledger.Ledger, chain *ChaincodeSupport) error
  ExecuteTransaction(ctxt context.Context, t *pb.Transaction) error
}

type DefaultChaincodeLifecycleManager struct{
  lgr *ledger.Ledger
  chain *ChaincodeSupport
}

func (lcm *DefaultChaincodeLifecycleManager) Init(lgr *ledger.Ledger, chain *ChaincodeSupport) error {
  lcm.lgr = lgr
  lcm.chain = chain
  return nil
}

func (lcm *DefaultChaincodeLifecycleManager) ExecuteTransaction(ctxt context.Context, t *pb.Transaction) error {
  if t.Type == pb.Transaction_CHAINCODE_DEPLOY {
    _, err := lcm.chain.DeployChaincode(ctxt, t)
    if err != nil {
      return fmt.Errorf("Failed to deploy chaincode spec(%s)", err)
    }

//     //launch and wait for ready
//     markTxBegin(lgr, t)
//     _, _, err = chain.LaunchChaincode(ctxt, t)
//     if err != nil {
//       markTxFinish(lgr, t, false)
//       return nil, fmt.Errorf("%s", err)
//     }
//     markTxFinish(lgr, t, true)
    }
    return nil
 }
//
// func getChaincodeInfo(lgr *ledger.Ledger, chaincodeName string) (*ChaincodeInfo, error) {
// 	buf, err := lgr.GetState(chaincodeInfoNameSpace, chaincodeName)
// 	if err != nil{
// 		return nil, err
// 	}
// 	if buf == nil {
// 		return nil, nil
// 	}
// 	chaincodeInfo := &ChaincodeInfo{}
// 	err = proto.Unmarshal(buf, chaincodeInfo)
// 	if err != nil{
// 		return nil, err
// 	}
// 	return chaincodeInfo, nil
// }
//
// func putChaincodeInfo(lgr *ledger.Ledger, chaincodeName string, info *ChaincodeInfo) error {
// 	buf, err := proto.Marshal(info)
// 	if err != nil {
// 		return err
// 	}
// 	return stub.PutState(chaincodeInfoNameSpacechaincodeName, buf)
// }
//
// func constructChaincodeBytesKey(chaincodeName string) string {
// 	return fmt.Sprintf("%s_%s", chaincodeName, "bytes")
// }
