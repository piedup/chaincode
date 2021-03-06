// Copyright 2017 Stratumn SAS. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storetestcases

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync/atomic"
	"testing"

	"github.com/stratumn/sdk/cs/cstesting"
	"github.com/stratumn/sdk/store"
	"github.com/stratumn/sdk/testutil"
)

// TestGetMapIDs tests what happens when you get map IDs with default
// pagination.
func (f Factory) TestGetMapIDs(t *testing.T) {
	a := f.initAdapter(t)
	defer f.free(a)

	for i := 0; i < store.DefaultLimit; i++ {
		for j := 0; j < store.DefaultLimit; j++ {
			s := cstesting.RandomSegment()
			s.Link.Meta["mapId"] = fmt.Sprintf("map%d", i)
			a.SaveSegment(s)
		}
	}

	slice, err := a.GetMapIDs(&store.MapFilter{Pagination: store.Pagination{Limit: store.DefaultLimit * store.DefaultLimit}})
	if err != nil {
		t.Fatalf("a.GetMapIDs(): err: %s", err)
	}

	if got, want := len(slice), store.DefaultLimit; got != want {
		t.Errorf("len(slice) = %d want %d", got, want)
	}

	for i := 0; i < store.DefaultLimit; i++ {
		mapID := fmt.Sprintf("map%d", i)
		if !testutil.ContainsString(slice, mapID) {
			t.Errorf("slice does not contain %q", mapID)
		}
	}
}

// TestGetMapIDsPagination tests what happens when you get map IDs with
// pagination.
func (f Factory) TestGetMapIDsPagination(t *testing.T) {
	a := f.initAdapter(t)
	defer f.free(a)

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			segment := cstesting.RandomSegment()
			segment.Link.Meta["mapId"] = fmt.Sprintf("map%d", i)
			a.SaveSegment(segment)
		}
	}

	slice, err := a.GetMapIDs(&store.MapFilter{Pagination: store.Pagination{Offset: 3, Limit: 5}})
	if err != nil {
		t.Fatalf("a.GetMapIDs(): err: %s", err)
	}

	if got, want := len(slice), 5; got != want {
		t.Errorf("len(slice) = %d want %d", got, want)
	}
}

// TestGetMapIDsEmpty tests what happens when you should get no map IDs.
func (f Factory) TestGetMapIDsEmpty(t *testing.T) {
	a := f.initAdapter(t)
	defer f.free(a)

	slice, err := a.GetMapIDs(&store.MapFilter{Pagination: store.Pagination{Offset: 100000, Limit: 5}})
	if err != nil {
		t.Fatalf("a.GetMapIDs(): err: %s", err)
	}

	if got, want := len(slice), 0; got != want {
		t.Errorf("len(slice) = %d want %d", got, want)
	}
}

// TestGetMapIDsByProcess tests what happens when you get map IDs filtered by process name.
func (f Factory) TestGetMapIDsByProcess(t *testing.T) {
	var processNames = [2]string{"Foo", "Bar"}
	a := f.initAdapter(t)
	defer f.free(a)

	for i := 0; i < store.DefaultLimit; i++ {
		for j := 0; j < store.DefaultLimit; j++ {
			s := cstesting.RandomSegment()
			s.Link.Meta["mapId"] = fmt.Sprintf("map%d", i)
			s.Link.Meta["process"] = processNames[i%2]
			a.SaveSegment(s)
		}
	}

	slice, err := a.GetMapIDs(&store.MapFilter{Pagination: store.Pagination{Limit: store.DefaultLimit * store.DefaultLimit}, Process: processNames[0]})
	if err != nil {
		t.Fatalf("a.GetMapIDsByProcess(): err: %s", err)
	}

	if got, want := len(slice), store.DefaultLimit/2; got != want {
		t.Errorf("len(slice) = %d want %d", got, want)
	}

	for i := 0; i < store.DefaultLimit; i += 2 {
		mapID := fmt.Sprintf("map%d", i)
		if !testutil.ContainsString(slice, mapID) {
			t.Errorf("slice does not contain %q", mapID)
		}
	}
}

// BenchmarkGetMapIDs benchmarks getting map IDs.
func (f Factory) BenchmarkGetMapIDs(b *testing.B, numSegments int, segmentFunc SegmentFunc, filterFunc MapFilterFunc) {
	a := f.initAdapterB(b)
	defer f.free(a)

	for i := 0; i < numSegments; i++ {
		a.SaveSegment(segmentFunc(b, numSegments, i))
	}

	filters := make([]*store.MapFilter, b.N)
	for i := 0; i < b.N; i++ {
		filters[i] = filterFunc(b, numSegments, i)
	}

	b.ResetTimer()
	log.SetOutput(ioutil.Discard)

	for i := 0; i < b.N; i++ {
		if s, err := a.GetMapIDs(filters[i]); err != nil {
			b.Fatal(err)
		} else if s == nil {
			b.Error("s = nil want []string")
		}
	}
}

// BenchmarkGetMapIDs100 benchmarks getting map IDs within 100 segments.
func (f Factory) BenchmarkGetMapIDs100(b *testing.B) {
	f.BenchmarkGetMapIDs(b, 100, RandomSegment, RandomPaginationOffset)
}

// BenchmarkGetMapIDs1000 benchmarks getting map IDs within 1000 segments.
func (f Factory) BenchmarkGetMapIDs1000(b *testing.B) {
	f.BenchmarkGetMapIDs(b, 1000, RandomSegment, RandomPaginationOffset)
}

// BenchmarkGetMapIDs10000 benchmarks getting map IDs within 10000 segments.
func (f Factory) BenchmarkGetMapIDs10000(b *testing.B) {
	f.BenchmarkGetMapIDs(b, 10000, RandomSegment, RandomPaginationOffset)
}

// BenchmarkGetMapIDsParallel benchmarks getting map IDs in parallel.
func (f Factory) BenchmarkGetMapIDsParallel(b *testing.B, numSegments int, segmentFunc SegmentFunc, filterFunc MapFilterFunc) {
	a := f.initAdapterB(b)
	defer f.free(a)

	for i := 0; i < numSegments; i++ {
		a.SaveSegment(segmentFunc(b, numSegments, i))
	}

	filters := make([]*store.MapFilter, b.N)
	for i := 0; i < b.N; i++ {
		filters[i] = filterFunc(b, numSegments, i)
	}

	var counter uint64

	b.ResetTimer()
	log.SetOutput(ioutil.Discard)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddUint64(&counter, 1) - 1)
			if s, err := a.GetMapIDs(filters[i]); err != nil {
				b.Error(err)
			} else if s == nil {
				b.Error("s = nil want []string")
			}
		}
	})
}

// BenchmarkGetMapIDs100Parallel benchmarks getting map IDs within 100 segments
// in parallel.
func (f Factory) BenchmarkGetMapIDs100Parallel(b *testing.B) {
	f.BenchmarkGetMapIDsParallel(b, 100, RandomSegment, RandomPaginationOffset)
}

// BenchmarkGetMapIDs1000Parallel benchmarks getting map IDs within 1000
// segments in parallel.
func (f Factory) BenchmarkGetMapIDs1000Parallel(b *testing.B) {
	f.BenchmarkGetMapIDsParallel(b, 1000, RandomSegment, RandomPaginationOffset)
}

// BenchmarkGetMapIDs10000Parallel benchmarks getting map IDs within 10000
// segments in parallel.
func (f Factory) BenchmarkGetMapIDs10000Parallel(b *testing.B) {
	f.BenchmarkGetMapIDsParallel(b, 10000, RandomSegment, RandomPaginationOffset)
}
