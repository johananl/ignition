// Copyright 2017 CoreOS, Inc.
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

package types

import (
	"reflect"
	"testing"

	"github.com/flatcar-linux/ignition/config/shared/errors"
	"github.com/flatcar-linux/ignition/config/util"
)

func TestModeValidate(t *testing.T) {
	type in struct {
		mode *int
	}
	type out struct {
		err error
	}

	tests := []struct {
		in  in
		out out
	}{
		{
			in:  in{mode: nil},
			out: out{},
		},
		{
			in:  in{mode: util.IntToPtr(0)},
			out: out{},
		},
		{
			in:  in{mode: util.IntToPtr(0644)},
			out: out{},
		},
		{
			in:  in{mode: util.IntToPtr(01755)},
			out: out{},
		},
		{
			in:  in{mode: util.IntToPtr(07777)},
			out: out{},
		},
		{
			in:  in{mode: util.IntToPtr(010000)},
			out: out{errors.ErrFileIllegalMode},
		},
	}

	for i, test := range tests {
		err := validateMode(test.in.mode)
		if !reflect.DeepEqual(test.out.err, err) {
			t.Errorf("#%d: bad err: want %v, got %v", i, test.out.err, err)
		}
	}
}
