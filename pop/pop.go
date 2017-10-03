package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/ledger/queryresult"
	sc "github.com/hyperledger/fabric/protos/peer"

	"github.com/stratumn/sdk/cs"
)

// Chaincode used to save segments to world state (putSegment) and query world state.
// Several queries have been implemented
// - getAllMaps returns all mapIDs
// - getMapsForProcess returns all mapIDs for a given process
// - getAllSegements returns all segments
// - getSegmentsForProcess returns all segments for a given process
// - getSegmentsForMap returns all segments for a given mapID
// - getSegment returns segments for a given linkHash

// Define the Smart Contract structure
type SmartContract struct {
}

// objectType used for composite keys
const (
	SegmentObjectType = "segment" // "segment", process, mapID, linkHash -> segment
	MapObjectType     = "map"     // "map", process, mapID -> nil
)

// linkHash -> (process, mapID) is used to get process and mapID when getting a specific segment
type SegmentIndex struct {
	Process string `json:"process"`
	MapID   string `json:"mapID"`
}

// Init method is called when the Smart Contract "pop" is instantiated by the blockchain network
func (s *SmartContract) Init(APIstub shim.ChaincodeStubInterface) sc.Response {
	return shim.Success(nil)
}

// Invoke method is called as a result of an application request to run the Smart Contract "pop"
func (s *SmartContract) Invoke(APIstub shim.ChaincodeStubInterface) sc.Response {
	// Retrieve the requested Smart Contract function and arguments
	function, args := APIstub.GetFunctionAndParameters()

	switch function {
	case "getMapsAll":
		return s.getMapsAll(APIstub, args)
	case "getMapsForProcess":
		return s.getMapsForProcess(APIstub, args)
	case "getSegmentsAll":
		return s.getSegmentsAll(APIstub, args)
	case "getSegmentsForProcess":
		return s.getSegmentsForProcess(APIstub, args)
	case "getSegmentsForMap":
		return s.getSegmentsForMap(APIstub, args)
	case "getSegment":
		return s.getSegment(APIstub, args)
	case "putSegment":
		return s.putSegment(APIstub, args)
	default:
		return shim.Error("Invalid Smart Contract function name.")
	}
}

// Saves map to world state
func (s *SmartContract) putMap(stub shim.ChaincodeStubInterface, segment *cs.Segment) error {
	process, mapID := segment.Link.GetProcess(), segment.Link.GetMapID()
	attrs := []string{process, mapID}
	ck, err := stub.CreateCompositeKey(MapObjectType, attrs)
	if err != nil {
		return err
	}

	// Store with composite key "map", process, mapID (query all maps for process)
	stub.PutState(ck, []byte(nil))

	// Index mapID -> process
	stub.PutState(mapID, []byte(process))
	return nil
}

// Get all mapIDs for a given process
func (s *SmartContract) getMapsForProcess(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	process := args[0]
	resultsIterator, err := stub.GetStateByPartialCompositeKey(MapObjectType, []string{process})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, getStringFromMapQueryResponse)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(resultBytes)
}

// Get all mapIDs
func (s *SmartContract) getMapsAll(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	resultsIterator, err := stub.GetStateByPartialCompositeKey(MapObjectType, []string{})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, getStringFromMapQueryResponse)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(resultBytes)
}

// Save segment to world state
func (s *SmartContract) putSegment(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	// Parse segment
	byteArgs := stub.GetArgs()
	segment := &cs.Segment{}
	if err := json.Unmarshal(byteArgs[1], segment); err != nil {
		return shim.Error("Could not parse segment")
	}

	// Validate segment
	if err := segment.Validate(); err != nil {
		return shim.Error(err.Error())
	}

	// Check has prevLinkHash if not create map else check prevLinkHash exists
	prevLinkHash := segment.Link.GetPrevLinkHashString()
	if prevLinkHash == "" {
		// Create map
		if err := s.putMap(stub, segment); err != nil {
			return shim.Error(err.Error())
		}
	} else {
		// Check previous segment exists
		response := s.getSegment(stub, []string{prevLinkHash})
		if response.Status == shim.ERROR {
			return shim.Error("Parent segment doesn't exist")
		}
	}

	//  Save segment
	process, mapID, linkHash := segment.Link.GetProcess(), segment.Link.GetMapID(), segment.GetLinkHashString()
	attrs := []string{process, mapID, linkHash}
	ck, err := stub.CreateCompositeKey(SegmentObjectType, attrs)
	if err != nil {
		return shim.Error(err.Error())
	}
	segmentBytes, err := json.Marshal(segment)
	if err != nil {
		return shim.Error(err.Error())
	}

	// Store with composite key "segment", process, mapID, linkHash
	stub.PutState(ck, segmentBytes)

	// Store in segment index linkHash -> (process, mapID)
	segmentIndex := SegmentIndex{process, mapID}
	segmentIndexBytes, err := json.Marshal(segmentIndex)
	if err != nil {
		return shim.Error(err.Error())
	}
	stub.PutState(linkHash, segmentIndexBytes)

	return shim.Success(nil)
}

// Get all segments
func (s *SmartContract) getSegmentsAll(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	resultsIterator, err := stub.GetStateByPartialCompositeKey(SegmentObjectType, []string{})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, getStringFromSegmentQueryResponse)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(resultBytes)
}

// Get all segments for a given process
func (s *SmartContract) getSegmentsForProcess(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	process := args[0]
	resultsIterator, err := stub.GetStateByPartialCompositeKey(SegmentObjectType, []string{process})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, getStringFromSegmentQueryResponse)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(resultBytes)
}

// Get all segments for a given mapID
func (s *SmartContract) getSegmentsForMap(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	mapId := args[0]
	processBytes, err := stub.GetState(mapId)
	if err != nil {
		return shim.Error(err.Error())
	}

	resultsIterator, err := stub.GetStateByPartialCompositeKey(SegmentObjectType, []string{string(processBytes), mapId})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, getStringFromSegmentQueryResponse)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(resultBytes)
}

// Get segment with corresponding linkHash
func (s *SmartContract) getSegment(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	linkHash := args[0]
	segmentIndexBytes, err := stub.GetState(linkHash)
	if err != nil {
		return shim.Error(err.Error())
	}
	if segmentIndexBytes == nil {
		return shim.Error("Segment does not exist")
	}

	segmentIndex := &SegmentIndex{}
	err = json.Unmarshal(segmentIndexBytes, segmentIndex)
	if err != nil {
		return shim.Error(err.Error())
	}

	attrs := []string{segmentIndex.Process, segmentIndex.MapID, linkHash}
	ck, err := stub.CreateCompositeKey(SegmentObjectType, attrs)
	if err != nil {
		return shim.Error(err.Error())
	}

	segmentBytes, err := stub.GetState(ck)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(segmentBytes)
}

// Saves result from range query or partial composite query into a bytes json array
func bytesFromResultsIterator(
	stub shim.ChaincodeStubInterface,
	resultsIterator shim.StateQueryIteratorInterface,
	getString func(stub shim.ChaincodeStubInterface, queryResponse *queryresult.KV) (string, error)) ([]byte, error) {

	var buffer bytes.Buffer
	buffer.WriteString("[")

	var resultString string
	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}

		resultString, err = getString(stub, queryResponse)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(resultString)

		bArrayMemberAlreadyWritten = true
	}

	buffer.WriteString("]")
	return buffer.Bytes(), nil
}

// Extracts mapID from queryResponse
func getStringFromMapQueryResponse(stub shim.ChaincodeStubInterface, queryResponse *queryresult.KV) (string, error) {
	_, attrs, err := stub.SplitCompositeKey(queryResponse.Key)
	if err != nil {
		return "", err
	}
	return "\"" + attrs[1] + "\"", nil
}

// Returns segment json as []byte from query response
func getStringFromSegmentQueryResponse(stub shim.ChaincodeStubInterface, queryResponse *queryresult.KV) (string, error) {
	return string(queryResponse.Value), nil
}

// main function starts up the chaincode in the container during instantiate
func main() {
	if err := shim.Start(new(SmartContract)); err != nil {
		fmt.Printf("Error starting SmartContract chaincode: %s", err)
	}
}
