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

package storetesting_test

import (
	"fmt"
	"log"

	"github.com/stratumn/sdk/store/storetesting"
)

// This example shows how to use a mock adapter.
func ExampleMockAdapter() {
	// Create a mock.
	m := storetesting.MockAdapter{}

	// Define a GetInfo function for our mock.
	m.MockGetInfo.Fn = func() (interface{}, error) {
		return map[string]string{
			"name": "test",
		}, nil
	}

	// Execute GetInfo on the mock.
	i, err := m.GetInfo()
	if err != nil {
		log.Fatal(err)
	}

	name := i.(map[string]string)["name"]

	// This is the number of times GetInfo was called.
	calledCount := m.MockGetInfo.CalledCount

	fmt.Printf("%s %d", name, calledCount)
	// Output: test 1
}
