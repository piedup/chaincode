package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/stratumn/sdk/cs"
)

func checkQuery(t *testing.T, stub *shim.MockStub, args [][]byte) []byte {
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

func checkInvoke(t *testing.T, stub *shim.MockStub, args [][]byte) {
	res := stub.MockInvoke("1", args)
	if res.Status != shim.OK {
		fmt.Println("Invoke", string(args[0]), "failed", string(res.Message))
		t.FailNow()
	}
}

func saveSegment(t *testing.T, stub *shim.MockStub, filepath string) *cs.Segment {
	segmentBytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Println("Could not load file", filepath)
	}

	checkInvoke(t, stub, [][]byte{[]byte("SaveSegment"), segmentBytes})

	segment := &cs.Segment{}
	err = json.Unmarshal(segmentBytes, segment)
	if err != nil {
		fmt.Println("Could not parse segment data")
	}

	return segment
}

func TestPop_newSegmentQuery(t *testing.T) {
	segmentFilter := "process=process&offset=10&limit=15&mapIds[]=id1&mapIds[]=id2&prevLinkHash=085fa4322980286778f896fe11c4f55c46609574d9188a3c96427c76b8500bcd&tags[]=tag1&tags[]=tag2"
	queryString, err := newSegmentQuery(segmentFilter)
	if err != nil {
		fmt.Println(err.Error())
	}

	segmentSelector := SegmentSelector{
		ObjectType:   "segment",
		Process:      "process",
		PrevLinkHash: "085fa4322980286778f896fe11c4f55c46609574d9188a3c96427c76b8500bcd",
		MapIds:       &MapIdsIn{[]string{"id1", "id2"}},
		Tags:         &TagsAll{[]string{"tag1", "tag2"}},
	}
	segmentQuery := SegmentQuery{
		Selector: segmentSelector,
		Limit:    15,
		Skip:     10,
	}

	r, err := json.Marshal(segmentQuery)
	if err != nil {
		fmt.Println(err.Error())
	}

	if queryString != string(r) {
		fmt.Println("Segment query json incorrect")
		t.FailNow()
	}
}

func TestPop_newMapQuery(t *testing.T) {
	mapFilter := "process=process&offset=10"
	queryString, _ := newMapQuery(mapFilter)
	mapSelector := MapSelector{
		ObjectType: "map",
		Process:    "process",
	}
	mapQuery := MapQuery{
		Selector: mapSelector,
		Skip:     10,
	}
	r, _ := json.Marshal(mapQuery)
	if queryString != string(r) {
		fmt.Println("Map query json incorrect")
		t.FailNow()
	}
}

func TestPop_SaveSegment(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	segment := saveSegment(t, stub, "./segment1.json")
	segmentBytes, _ := json.Marshal(segment)
	payload := checkQuery(t, stub, [][]byte{[]byte("GetSegment"), []byte(segment.GetLinkHashString())})

	if string(segmentBytes) != string(payload) {
		fmt.Println("Segment not saved into database")
		t.FailNow()
	}
}

func TestPop_SaveSegmentMissingParent(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	segmentBytes, err := ioutil.ReadFile("./segment2.json")
	if err != nil {
		fmt.Println("Could not load file ./segment2.json")
	}

	res := stub.MockInvoke("1", [][]byte{[]byte("SaveSegment"), segmentBytes})
	if res.Status != shim.ERROR {
		fmt.Println("SaveSegment should have failed")
		t.FailNow()
	} else {
		if res.Message != "Parent segment doesn't exist" {
			fmt.Println("Failed with error", res.Message, "expected", "Parent segment doesn't exist")
			t.FailNow()
		}
	}
}

func TestPop_SaveSegmentIncorrect(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	res := stub.MockInvoke("1", [][]byte{[]byte("SaveSegment"), []byte("")})
	if res.Status != shim.ERROR {
		fmt.Println("SaveSegment should have failed")
		t.FailNow()
	} else {
		if res.Message != "Could not parse segment" {
			fmt.Println("Failed with error", res.Message, "expected", "Could not parse segment")
			t.FailNow()
		}
	}
}

func TestPop_GetSegmentDoesNotExist(t *testing.T) {
	cc := new(SmartContract)
	stub := shim.NewMockStub("pop", cc)

	res := stub.MockInvoke("1", [][]byte{[]byte("GetSegment"), []byte("")})
	if res.Status != shim.ERROR {
		fmt.Println("GetSegment should have failed")
		t.FailNow()
	} else {
		if res.Message != "Segment does not exist" {
			fmt.Println("Failed with error", res.Message, "expected", "Segment does not exist")
			t.FailNow()
		}
	}
}
