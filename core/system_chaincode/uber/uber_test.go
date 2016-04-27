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
	"os"
	"testing"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

func TestUberCCDeploy(t *testing.T) {
	h := newUberTestHelper(t)
	h.startChaincodeRuntimeWithGRPC()
	defer h.stopGRPCServer()

	ccName := h.deployViaUber("github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02", "init", []string{"a", "100", "b", "200"})
	t.Logf("Deployed chaincode - example02")

	// validate state set by init
	h.assertState(ccName, "a", []byte("100"))
	h.assertState(ccName, "b", []byte("200"))
	t.Logf("Validated state after init of chaincode - example02")

	// invoke
	h.executeInvoke(ccName, "", []string{"a", "b", "10"})
	t.Logf("invoked a tx on chaincode - example02")

	// validate state modified by invoke
	h.assertState(ccName, "a", []byte("90"))
	h.assertState(ccName, "b", []byte("210"))
	t.Logf("Validated state after invoke on chaincode - example02")
}

func TestUberCCUpgrade(t *testing.T) {
	h := newUberTestHelper(t)
	h.startChaincodeRuntimeWithGRPC()
	defer h.stopGRPCServer()
	ccName := h.deployViaUber("github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02", "init", []string{"a", "100", "b", "200"})
	t.Logf("Deployed chaincode - example02")

	// validate state set by init
	h.assertState(ccName, "a", []byte("100"))
	h.assertState(ccName, "b", []byte("200"))
	t.Logf("Validated state after init of chaincode - example02")

	// invoke
	h.executeInvoke(ccName, "", []string{"a", "b", "10"})
	t.Logf("invoked a tx on chaincode - example02")

	// validate state modified by invoke
	h.assertState(ccName, "a", []byte("90"))
	h.assertState(ccName, "b", []byte("210"))
	t.Logf("Validated state after invoke on chaincode - example02")

	// upgrade chaincode
	upgradedCCName := h.upgradeViaUber(ccName, "github.com/hyperledger/fabric/core/system_chaincode/uber/test/chaincode_example02_upgraded", "init", []string{"a", "b", "2"})
	t.Logf("Upgraded chaincode - example02_upgraded from chaincode - example02")

	// validate state copied from parent and modified by init
	h.assertState(upgradedCCName, "a", []byte("92"))
	h.assertState(upgradedCCName, "b", []byte("212"))
	t.Logf("Validated state after init of chaincode - example02_upgraded")

	// invoke on new chaicode
	h.executeInvoke(upgradedCCName, "", []string{"b", "a", "10"})
	t.Logf("invoked a tx on chaincode - example02_upgraded")

	// validate state modified by invoke on new chaicode
	h.assertState(upgradedCCName, "a", []byte("102"))
	h.assertState(upgradedCCName, "b", []byte("202"))
	t.Logf("Validated state after invoke on chaincode - example02_upgraded")
}

func TestMain(m *testing.M) {
	setupTestConfig()
	viper.Set("ledger.blockchain.deploy-system-chaincode", "false")
	viper.Set("validator.validity-period.verification", "false")
	os.Exit(m.Run())
}

// SetupTestConfig setup the config during test execution
func setupTestConfig() {
	viper.SetConfigName("uber_test")          // name of config file (without extension)
	viper.AddConfigPath(".")            // path to look for the config file in
	err := viper.ReadInConfig()          // Find and read the config file
	if err != nil {                      // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	setupTestLogging()
}

func setupTestLogging() {
  var formatter = logging.MustStringFormatter(
    `%{color} [%{module}] %{shortfunc} [%{shortfile}] -> %{level:.4s} %{id:03x}%{color:reset} %{message}`,
  )
  logging.SetFormatter(formatter)
	logging.SetLevel(logging.ERROR, "main")
	logging.SetLevel(logging.ERROR, "server")
	logging.SetLevel(logging.ERROR, "peer")
  logging.SetLevel(logging.ERROR, "ledger")
}
