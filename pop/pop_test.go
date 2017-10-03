package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stratumn/sdk/cs"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

func checkInit(t *testing.T, stub *shim.MockStub) {
	res := stub.MockInit("1", [][]byte{[]byte("init")})
	if res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}
}

func checkInvoke(t *testing.T, stub *shim.MockStub, args [][]byte) {
	res := stub.MockInvoke("1", args)
	if res.Status != shim.OK {
		fmt.Println("Invoke", string(args[0]), "failed", string(res.Message))
		t.FailNow()
	}
}

func checkQuery(t *testing.T, stub *shim.MockStub, args [][]byte) []byte {
	// args[0] should be function name
	res := stub.MockInvoke("1", args)
	if res.Status != shim.OK {
		fmt.Println("Query failed", string(res.Message))
		t.FailNow()
	}
	if res.Payload == nil {
		fmt.Println("Query failed to get value")
		t.FailNow()
	}

	return res.Payload
}

func saveSegment(t *testing.T, stub *shim.MockStub, filepath string) *cs.Segment {
	segmentBytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Println("Could not load file", filepath)
	}

	checkInvoke(t, stub, [][]byte{[]byte("putSegment"), segmentBytes})

	segment := &cs.Segment{}
	err = json.Unmarshal(segmentBytes, segment)
	if err != nil {
		fmt.Println("Could not parse segment data")
	}

	return segment
}

// Initialize chaincode
func TestPop_Init(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	checkInit(t, stub)
}

// Saving segment
func TestPop_PutSegment(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	checkInit(t, stub)

	segment := saveSegment(t, stub, "./segment1.json")
	segmentBytes, _ := json.Marshal(segment)

	// getMapsAll
	payload := checkQuery(t, stub, [][]byte{[]byte("getMapsAll")})
	if string(payload) != "[\""+segment.Link.GetMapID()+"\"]" {
		fmt.Println("getMapsAll failed")
		t.FailNow()
	}

	// getMapsForProcess
	payload = checkQuery(t, stub, [][]byte{[]byte("getMapsForProcess"), []byte(segment.Link.GetProcess())})
	if string(payload) != "[\""+segment.Link.GetMapID()+"\"]" {
		fmt.Println("getMapsForProcess failed")
		t.FailNow()
	}

	// getSegmentsAll
	payload = checkQuery(t, stub, [][]byte{[]byte("getSegmentsAll")})
	if string(payload) != "["+string(segmentBytes)+"]" {
		fmt.Println("getSegmentsAll failed")
		t.FailNow()
	}

	// getSegmentsForProcess
	payload = checkQuery(t, stub, [][]byte{[]byte("getSegmentsForProcess"), []byte(segment.Link.GetProcess())})
	if string(payload) != "["+string(segmentBytes)+"]" {
		fmt.Println("getSegmentsForProcess failed")
		t.FailNow()
	}

	// getSegmentsForMap
	payload = checkQuery(t, stub, [][]byte{[]byte("getSegmentsForMap"), []byte(segment.Link.GetMapID())})
	if string(payload) != "["+string(segmentBytes)+"]" {
		fmt.Println("getSegmentsForMap failed")
		t.FailNow()
	}

	// getSegment
	payload = checkQuery(t, stub, [][]byte{[]byte("getSegment"), []byte(segment.GetLinkHashString())})
	if string(payload) != string(segmentBytes) {
		fmt.Println("getSegment failed")
		t.FailNow()
	}
}

// Saving two segments
func TestPop_PutTwoSegments(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	checkInit(t, stub)

	segment1 := saveSegment(t, stub, "./segment1.json")
	segment2 := saveSegment(t, stub, "./segment2.json")
	segment3 := saveSegment(t, stub, "./segment3.json")

	segment1Bytes, _ := json.Marshal(segment1)
	segment2Bytes, _ := json.Marshal(segment2)

	// getSegmentsForMap
	payload := checkQuery(t, stub, [][]byte{[]byte("getSegmentsForMap"), []byte(segment1.Link.GetMapID())})
	if string(payload) != "["+string(segment1Bytes)+","+string(segment2Bytes)+"]" {
		fmt.Println("getSegmentsForMap failed")
		t.FailNow()
	}

	// getMapsAll
	payload = checkQuery(t, stub, [][]byte{[]byte("getMapsAll")})
	if string(payload) != "[\""+string(segment1.Link.GetMapID())+"\",\""+string(segment3.Link.GetMapID())+"\"]" {
		fmt.Println("getAllMaps failed")
		t.FailNow()
	}
}

// Saving segment when parent segment not saved (should fail)
func TestPop_MissingParent(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	checkInit(t, stub)

	segmentBytes, err := ioutil.ReadFile("./segment2.json")
	if err != nil {
		fmt.Println("Could not load file ./segment2.json")
	}

	res := stub.MockInvoke("1", [][]byte{[]byte("putSegment"), segmentBytes})
	if res.Status != shim.ERROR {
		fmt.Println("putSegment should have failed")
		t.FailNow()
	} else {
		if res.Message != "Parent segment doesn't exist" {
			fmt.Println("Failed with error", res.Message, "expected", "Parent segment doesn't exist")
			t.FailNow()
		}
	}
}

// Saving wrongly formatted segment (should fail)
func TestPop_IncorrectSegment(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	checkInit(t, stub)
	res := stub.MockInvoke("1", [][]byte{[]byte("putSegment"), []byte("")})
	if res.Status != shim.ERROR {
		fmt.Println("putSegment should have failed")
		t.FailNow()
	} else {
		if res.Message != "Could not parse segment" {
			fmt.Println("Failed with error", res.Message, "expected", "Could not parse segment")
			t.FailNow()
		}
	}
}

// Getting non existent segment (should fail)
func TestPop_GetNonExistingSegment(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	checkInit(t, stub)
	res := stub.MockInvoke("1", [][]byte{[]byte("getSegment"), []byte("")})
	if res.Status != shim.ERROR {
		fmt.Println("getSegment should have failed")
		t.FailNow()
	} else {
		if res.Message != "Segment does not exist" {
			fmt.Println("Failed with error", res.Message, "expected", "Segment does not exist")
			t.FailNow()
		}
	}
}
