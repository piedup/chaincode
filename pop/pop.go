package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/stratumn/sdk/store"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	sc "github.com/hyperledger/fabric/protos/peer"

	"github.com/stratumn/sdk/cs"
	"github.com/stratumn/sdk/types"
)

// Pagination functionality (limit & skip) is implemented in CouchDB but not in Hyperledger Fabric (FAB-2809 and FAB-5369).
// Creating an index in CouchDB:
// curl -i -X POST -H "Content-Type: application/json" -d "{\"index\":{\"fields\":[\"chaincodeid\",\"data.docType\",\"data.id\"]},\"name\":\"indexOwner\",\"ddoc\":\"indexOwnerDoc\",\"type\":\"json\"}" http://localhost:5984/mychannel/_index

// SmartContract defines chaincode logic
type SmartContract struct {
}

// ObjectType used in CouchDB documents
const (
	ObjectTypeSegment = "segment"
	ObjectTypeMap     = "map"
)

// MapDoc is used to store maps in CouchDB
type MapDoc struct {
	ObjectType string `json:"docType"`
	ID         string `json:"id"`
	Process    string `json:"process"`
}

// SegmentDoc is used to store segments in CouchDB
type SegmentDoc struct {
	ObjectType string     `json:"docType"`
	ID         string     `json:"id"`
	Segment    cs.Segment `json:"segment"`
}

// MapSelector used in MapQuery
type MapSelector struct {
	ObjectType string `json:"docType"`
	Process    string `json:"process,omitempty"`
}

// MapQuery used in CouchDB rich queries
type MapQuery struct {
	Selector MapSelector `json:"selector,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	Skip     int         `json:"skip,omitempty"`
}

func newMapQuery(mapFilter string) (string, error) {
	filter, err := parseMapFilter(mapFilter)
	if err != nil {
		return "", err
	}

	mapSelector := MapSelector{}
	mapSelector.ObjectType = ObjectTypeMap

	if filter.Process != "" {
		mapSelector.Process = filter.Process
	}

	mapQuery := MapQuery{
		mapSelector,
		filter.Pagination.Limit,
		filter.Pagination.Offset,
	}

	queryBytes, err := json.Marshal(mapQuery)
	if err != nil {
		return "", err
	}

	return string(queryBytes), nil
}

// SegmentSelector used in SegmentQuery
type SegmentSelector struct {
	ObjectType   string    `json:"docType"`
	LinkHash     string    `json:"id,omitempty"`
	PrevLinkHash string    `json:"segment.link.meta.prevLinkHash,omitempty"`
	Process      string    `json:"segment.link.meta.process,omitempty"`
	MapIds       *MapIdsIn `json:"segment.link.meta.mapId,omitempty"`
	Tags         *TagsAll  `json:"segment.link.meta.tags,omitempty"`
}

// MapIdsIn specifies that segment mapId should be in specified list
type MapIdsIn struct {
	MapIds []string `json:"$in,omitempty"`
}

// TagsAll specifies all tags in specified list should be in segment tags
type TagsAll struct {
	Tags []string `json:"$all,omitempty"`
}

// SegmentQuery used in CouchDB rich queries
type SegmentQuery struct {
	Selector SegmentSelector `json:"selector,omitempty"`
	Limit    int             `json:"limit,omitempty"`
	Skip     int             `json:"skip,omitempty"`
}

func newSegmentQuery(segmentFilter string) (string, error) {
	filter, err := parseSegmentFilter(segmentFilter)
	if err != nil {
		return "", err
	}

	segmentSelector := SegmentSelector{}
	segmentSelector.ObjectType = ObjectTypeSegment

	if filter.PrevLinkHash != nil {
		segmentSelector.PrevLinkHash = filter.PrevLinkHash.String()
	}
	if filter.Process != "" {
		segmentSelector.Process = filter.Process
	}
	if len(filter.MapIDs) > 0 {
		segmentSelector.MapIds = &MapIdsIn{filter.MapIDs}
	} else {
		segmentSelector.Tags = nil
	}
	if len(filter.Tags) > 0 {
		segmentSelector.Tags = &TagsAll{filter.Tags}
	} else {
		segmentSelector.Tags = nil
	}

	segmentQuery := SegmentQuery{
		Selector: segmentSelector,
		Limit:    filter.Pagination.Limit,
		Skip:     filter.Pagination.Offset,
	}

	queryBytes, err := json.Marshal(segmentQuery)
	if err != nil {
		return "", err
	}

	return string(queryBytes), nil
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
	case "GetSegment":
		return s.GetSegment(APIstub, args)
	case "FindSegments":
		return s.FindSegments(APIstub, args)
	case "GetMapIDs":
		return s.GetMapIDs(APIstub, args)
	case "SaveSegment":
		return s.SaveSegment(APIstub, args)
	default:
		return shim.Error("Invalid Smart Contract function name.")
	}
}

// SaveMap saves map into CouchDB using map document
func (s *SmartContract) SaveMap(stub shim.ChaincodeStubInterface, segment *cs.Segment) error {
	mapDoc := MapDoc{
		ObjectTypeMap,
		segment.Link.GetMapID(),
		segment.Link.GetProcess(),
	}
	mapDocBytes, err := json.Marshal(mapDoc)
	if err != nil {
		return err
	}
	if err := stub.PutState(segment.Link.GetMapID(), mapDocBytes); err != nil {
		return err
	}

	return nil
}

// SaveSegment saves segment into CouchDB using segment document
func (s *SmartContract) SaveSegment(stub shim.ChaincodeStubInterface, args []string) sc.Response {
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
		if err := s.SaveMap(stub, segment); err != nil {
			return shim.Error(err.Error())
		}
	} else {
		// Check previous segment exists
		response := s.GetSegment(stub, []string{prevLinkHash})
		if response.Status == shim.ERROR {
			return shim.Error("Parent segment doesn't exist")
		}
	}

	//  Save segment
	segmentDoc := SegmentDoc{
		ObjectTypeSegment,
		segment.GetLinkHashString(),
		*segment,
	}
	segmentDocBytes, err := json.Marshal(segmentDoc)
	if err != nil {
		return shim.Error(err.Error())
	}
	if err := stub.PutState(segment.GetLinkHashString(), segmentDocBytes); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

// GetSegment gets segment for given linkHash
func (s *SmartContract) GetSegment(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	segmentDocBytes, err := stub.GetState(args[0])
	if err != nil {
		return shim.Error(err.Error())
	}
	if segmentDocBytes == nil {
		return shim.Error("Segment does not exist")
	}

	segmentBytes, err := extractSegment(segmentDocBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(segmentBytes)
}

// FindSegments returns segments that match specified segment filter
func (s *SmartContract) FindSegments(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	queryString, err := newSegmentQuery(args[0])
	if err != nil {
		return shim.Error("Segment filter format incorrect")
	}

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return shim.Error(err.Error())
	}

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, extractSegment)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(resultBytes)
}

// GetMapIDs returns mapIDs for maps that match specified map filter
func (s *SmartContract) GetMapIDs(stub shim.ChaincodeStubInterface, args []string) sc.Response {
	queryString, err := newMapQuery(args[0])
	if err != nil {
		return shim.Error("Map filter format incorrect")
	}

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return shim.Error(err.Error())
	}

	resultBytes, err := bytesFromResultsIterator(stub, resultsIterator, extractMapID)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(resultBytes)
}

// Saves result from range query or partial composite query into a bytes json array
func bytesFromResultsIterator(stub shim.ChaincodeStubInterface, resultsIterator shim.StateQueryIteratorInterface, extract func([]byte) ([]byte, error)) ([]byte, error) {

	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}

		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		resultBytes, err := extract(queryResponse.Value)
		if err != nil {
			return nil, err
		}
		buffer.Write(resultBytes)

		bArrayMemberAlreadyWritten = true
	}

	buffer.WriteString("]")
	return buffer.Bytes(), nil
}

func extractSegment(segmentDocBytes []byte) ([]byte, error) {
	segmentDoc := &SegmentDoc{}
	if err := json.Unmarshal(segmentDocBytes, segmentDoc); err != nil {
		return nil, err
	}
	segmentBytes, err := json.Marshal(segmentDoc.Segment)
	if err != nil {
		return nil, err
	}
	return segmentBytes, nil
}

func extractMapID(mapDocBytes []byte) ([]byte, error) {
	mapDoc := &MapDoc{}
	if err := json.Unmarshal(mapDocBytes, mapDoc); err != nil {
		return nil, err
	}
	return []byte("\"" + mapDoc.ID + "\""), nil
}

func parseSegmentFilter(q string) (*store.SegmentFilter, error) {
	v, err := url.ParseQuery(q)
	if err != nil {
		return nil, err
	}

	var (
		mapIDs          = append(v["mapIds[]"], v["mapIds%5B%5D"]...)
		process         = v.Get("process")
		prevLinkHashStr = v.Get("prevLinkHash")
		tags            = append(v["tags[]"], v["tags%5B%5D"]...)
		prevLinkHash    *types.Bytes32
	)

	if prevLinkHashStr != "" {
		prevLinkHash, err = types.NewBytes32FromString(prevLinkHashStr)
		if err != nil {
			return nil, err
		}
	}

	pagination, err := parsePagination(q)

	return &store.SegmentFilter{
		Pagination:   *pagination,
		MapIDs:       mapIDs,
		Process:      process,
		PrevLinkHash: prevLinkHash,
		Tags:         tags,
	}, nil
}

func parseMapFilter(q string) (*store.MapFilter, error) {
	v, _ := url.ParseQuery(q)
	pagination, _ := parsePagination(q)

	return &store.MapFilter{
		Pagination: *pagination,
		Process:    v.Get("process"),
	}, nil
}

func parsePagination(q string) (*store.Pagination, error) {
	v, err := url.ParseQuery(q)

	offsetstr := v.Get("offset")
	offset := 0
	if offsetstr != "" {
		if offset, err = strconv.Atoi(offsetstr); err != nil || offset < 0 {
			return nil, err
		}
	}

	limitstr := v.Get("limit")
	limit := 0
	if limitstr != "" {
		if limit, err = strconv.Atoi(limitstr); err != nil || limit < 0 {
			return nil, err
		}
	}

	return &store.Pagination{
		Offset: offset,
		Limit:  limit,
	}, nil
}

// main function starts up the chaincode in the container during instantiate
func main() {
	if err := shim.Start(new(SmartContract)); err != nil {
		fmt.Printf("Error starting SmartContract chaincode: %s", err)
	}
}
