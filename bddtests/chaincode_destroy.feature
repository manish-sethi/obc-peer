#
# Test chaincode destroy feature
#
# Tags that can be used and will affect test internals:
#
#  @doNotDecompose will NOT decompose the named compose_yaml after scenario ends.  Useful for setting up environment and reviewing after scenario.
#
#  @chaincodeImagesUpToDate use this if all scenarios chaincode images are up to date, and do NOT require building.  BE SURE!!!

#@chaincodeImagesUpToDate
Feature: Destroying a previously deployed chaincode
    As a HyperLedger developer
    I want to be able to destroy a chaincode

#    @doNotDecompose
#    @wip
	Scenario: destroy chaincode with 5 peers, issue #1133
	  Given we compose "docker-compose-5.yml"
	  And I wait "1" seconds
	  When requesting "/chain" from "vp0"
	  Then I should get a JSON response with "height" = "1"

	  When I deploy chaincode "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02" with ctor "init" to "vp0"
	     | arg1 |  arg2 | arg3 | arg4 |
	     |  a   |  100  |  b   |  200 |
	  Then I should have received a chaincode name
	  Then I wait up to "60" seconds for transaction to be committed to all peers

    When I query chaincode via RESTendpoint-chaincode function name "query" on all peers
       |arg1|
       |  a |
	  Then I should get a JSON response from all peers with path "result.message" = "100"

	  When I destroy chaincode on "vp0"
	  Then I should have received a transactionID
	  Then I wait up to "60" seconds for transaction to be committed to all peers

    When I query chaincode via RESTendpoint-chaincode function name "query" on all peers
       |arg1|
       |  a |
    Then I should get a JSON response from all peers with path "error.code" = "-32003"
