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
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/core/ledger"
	pb "github.com/hyperledger/fabric/protos"
)

const (
	chaincodeInfoNameSpace = "UBER"
)

func Execute(ctxt context.Context, chain *ChaincodeSupport, t *pb.Transaction) ([]byte, error) {
	var err error
	var lgr *ledger.Ledger

	// get a handle to ledger to mark the begin/finish of a tx
	lgr, err = ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get handle to ledger (%s)", err)
	}

	if secHelper := chain.getSecHelper(); nil != secHelper {
		var err error
		t, err = secHelper.TransactionPreExecution(t)
		// Note that t is now decrypted and is a deep clone of the original input t
		if nil != err {
			return nil, err
		}
	}

	// Deploy
	if t.Type == pb.Transaction_CHAINCODE_DEPLOY {
		cds, err := extractChaincodeDeploymentSpec(t)
		if err != nil {
			return nil, err
		}
		ccName := cds.ChaincodeSpec.ChaincodeID.Name
		ccExists, err := chaincodeExists(lgr, ccName)
		if err != nil {
			return nil, err
		}
		if ccExists {
			return nil, fmt.Errorf("Chaincode [%s] already exists", ccName)
		}
		_, err = chain.Deploy(ctxt, t)
		if err != nil {
			return nil, fmt.Errorf("Failed to deploy chaincode spec(%s)", err)
		}

		//launch and wait for ready
		markTxBegin(lgr, t)
		_, _, err = chain.Launch(ctxt, t)
		if err != nil {
			markTxFinish(lgr, t, false)
			return nil, fmt.Errorf("%s", err)
		}
		ccInfo := &ChaincodeInfo{ParentChaincodeName: "__"}
		err = putChaincodeInfo(lgr, ccName, ccInfo)
		if err != nil {
			markTxFinish(lgr, t, false)
			return nil, err
		}
		markTxFinish(lgr, t, true)
	} else if t.Type == pb.Transaction_CHAINCODE_UPGRADE {
		cds, err := extractChaincodeDeploymentSpec(t)
		if err != nil {
			return nil, err
		}
		ccName := cds.ChaincodeSpec.ChaincodeID.Name
		parentCCName := cds.ChaincodeSpec.ChaincodeID.Parent
		parentCCExists, err := chaincodeExists(lgr, parentCCName)
		if err != nil {
			return nil, err
		}
		if !parentCCExists {
			return nil, fmt.Errorf("Parent Chaincode [%s] does not exist. Cannot upgrade chaincode [%s]", parentCCName, ccName)
		}
		_, err = chain.Deploy(ctxt, t)
		if err != nil {
			return nil, fmt.Errorf("Failed to deploy chaincode spec(%s)", err)
		}

		markTxBegin(lgr, t)
		err = lgr.CopyState(parentCCName, ccName)
		if err != nil {
			markTxFinish(lgr, t, false)
			return nil, err
		}
		//launch and wait for ready
		_, _, err = chain.Launch(ctxt, t)
		if err != nil {
			chaincodeLogger.Debug("Error...chain.LaunchChaincode:%s", err)
			markTxFinish(lgr, t, false)
			return nil, fmt.Errorf("%s", err)
		}
		ccInfo := &ChaincodeInfo{ParentChaincodeName: parentCCName}
		err = putChaincodeInfo(lgr, ccName, ccInfo)
		if err != nil {
			chaincodeLogger.Debug("Error...putChaincodeInfo:%s", err)
			markTxFinish(lgr, t, false)
			return nil, err
		}
		chaincodeLogger.Debug("Finishing Tx :-)")
		markTxFinish(lgr, t, true)
	} else if t.Type == pb.Transaction_CHAINCODE_INVOKE || t.Type == pb.Transaction_CHAINCODE_QUERY {
		//will launch if necessary (and wait for ready)
		cID, cMsg, err := chain.Launch(ctxt, t)
		if err != nil {
			return nil, fmt.Errorf("Failed to launch chaincode spec(%s)", err)
		}

		//this should work because it worked above...
		chaincode := cID.Name

		if err != nil {
			return nil, fmt.Errorf("Failed to stablish stream to container %s", chaincode)
		}

		// TODO: Need to comment next line and uncomment call to getTimeout, when transaction blocks are being created
		timeout := time.Duration(30000) * time.Millisecond
		//timeout, err := getTimeout(cID)

		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve chaincode spec(%s)", err)
		}

		var ccMsg *pb.ChaincodeMessage
		if t.Type == pb.Transaction_CHAINCODE_INVOKE {
			ccMsg, err = createTransactionMessage(t.Uuid, cMsg)
			if err != nil {
				return nil, fmt.Errorf("Failed to transaction message(%s)", err)
			}
		} else {
			ccMsg, err = createQueryMessage(t.Uuid, cMsg)
			if err != nil {
				return nil, fmt.Errorf("Failed to query message(%s)", err)
			}
		}

		markTxBegin(lgr, t)
		resp, err := chain.Execute(ctxt, chaincode, ccMsg, timeout, t)
		if err != nil {
			// Rollback transaction
			markTxFinish(lgr, t, false)
			return nil, fmt.Errorf("Failed to execute transaction or query(%s)", err)
		} else if resp == nil {
			// Rollback transaction
			markTxFinish(lgr, t, false)
			return nil, fmt.Errorf("Failed to receive a response for (%s)", t.Uuid)
		} else {
			if resp.Type == pb.ChaincodeMessage_COMPLETED || resp.Type == pb.ChaincodeMessage_QUERY_COMPLETED {
				// Success
				markTxFinish(lgr, t, true)
				return resp.Payload, nil
			} else if resp.Type == pb.ChaincodeMessage_ERROR || resp.Type == pb.ChaincodeMessage_QUERY_ERROR {
				// Rollback transaction
				markTxFinish(lgr, t, false)
				return nil, fmt.Errorf("Transaction or query returned with failure: %s", string(resp.Payload))
			}
			markTxFinish(lgr, t, false)
			return resp.Payload, fmt.Errorf("receive a response for (%s) but in invalid state(%d)", t.Uuid, resp.Type)
		}

	} else {
		err = fmt.Errorf("Invalid transaction type %s", t.Type.String())
	}
	return nil, err
}

//ExecuteTransactions - will execute transactions on the array one by one
//will return an array of errors one for each transaction. If the execution
//succeeded, array element will be nil. returns state hash
func ExecuteTransactions(ctxt context.Context, cname ChainName, xacts []*pb.Transaction) ([]byte, []error) {
	var chain = GetChain(cname)
	if chain == nil {
		// TODO: We should never get here, but otherwise a good reminder to better handle
		panic(fmt.Sprintf("[ExecuteTransactions]Chain %s not found\n", cname))
	}
	errs := make([]error, len(xacts)+1)
	for i, t := range xacts {
		_, errs[i] = Execute(ctxt, chain, t)
	}
	ledger, hasherr := ledger.GetLedger()
	var statehash []byte
	if hasherr == nil {
		statehash, hasherr = ledger.GetTempStateHash()
	}
	errs[len(errs)-1] = hasherr
	return statehash, errs
}

// GetSecureContext returns the security context from the context object or error
// Security context is nil if security is off from core.yaml file
// func GetSecureContext(ctxt context.Context) (crypto.Peer, error) {
// 	var err error
// 	temp := ctxt.Value("security")
// 	if nil != temp {
// 		if secCxt, ok := temp.(crypto.Peer); ok {
// 			return secCxt, nil
// 		}
// 		err = errors.New("Failed to convert security context type")
// 	}
// 	return nil, err
// }

var errFailedToGetChainCodeSpecForTransaction = errors.New("Failed to get ChainCodeSpec from Transaction")

func getTimeout(cID *pb.ChaincodeID) (time.Duration, error) {
	ledger, err := ledger.GetLedger()
	if err == nil {
		chaincodeID := cID.Name
		txUUID, err := ledger.GetState(chaincodeID, "github.com_openblockchain_obc-peer_chaincode_id", true)
		if err == nil {
			tx, err := ledger.GetTransactionByUUID(string(txUUID))
			if err == nil {
				chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{}
				proto.Unmarshal(tx.Payload, chaincodeDeploymentSpec)
				chaincodeSpec := chaincodeDeploymentSpec.GetChaincodeSpec()
				timeout := time.Duration(time.Duration(chaincodeSpec.Timeout) * time.Millisecond)
				return timeout, nil
			}
		}
	}

	return -1, errFailedToGetChainCodeSpecForTransaction
}

func markTxBegin(ledger *ledger.Ledger, t *pb.Transaction) {
	if t.Type == pb.Transaction_CHAINCODE_QUERY {
		return
	}
	ledger.TxBegin(t.Uuid)
}

func markTxFinish(ledger *ledger.Ledger, t *pb.Transaction, successful bool) {
	if t.Type == pb.Transaction_CHAINCODE_QUERY {
		return
	}
	ledger.TxFinished(t.Uuid, successful)
}

func extractChaincodeDeploymentSpec(tx *pb.Transaction) (*pb.ChaincodeDeploymentSpec, error) {
	cds := &pb.ChaincodeDeploymentSpec{}
	err := proto.Unmarshal(tx.Payload, cds)
	if err != nil {
		return nil, err
	}
	return cds, nil
}

func chaincodeExists(lgr *ledger.Ledger, chaincodeName string) (bool, error) {
	ccInfo, err := getChaincodeInfo(lgr, chaincodeName)
	if err != nil {
		return false, err
	}
	return ccInfo != nil, nil
}

func getChaincodeInfo(lgr *ledger.Ledger, chaincodeName string) (*ChaincodeInfo, error) {
	buf, err := lgr.GetState(chaincodeInfoNameSpace, chaincodeName, false)
	if err != nil {
		return nil, err
	}
	if buf == nil {
		return nil, nil
	}
	chaincodeInfo := &ChaincodeInfo{}
	err = proto.Unmarshal(buf, chaincodeInfo)
	if err != nil {
		return nil, err
	}
	return chaincodeInfo, nil
}

func putChaincodeInfo(lgr *ledger.Ledger, chaincodeName string, info *ChaincodeInfo) error {
	buf, err := proto.Marshal(info)
	if err != nil {
		return err
	}
	return lgr.SetState(chaincodeInfoNameSpace, chaincodeName, buf)
}

func constructChaincodeBytesKey(chaincodeName string) string {
	return fmt.Sprintf("%s_%s", chaincodeName, "bytes")
}
