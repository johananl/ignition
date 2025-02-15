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

package files

import (
	"github.com/flatcar-linux/ignition/tests/register"
	"github.com/flatcar-linux/ignition/tests/types"
)

func init() {
	register.Register(register.PositiveTest, CreateFileOnRoot())
	register.Register(register.PositiveTest, UserGroupByID())
	register.Register(register.PositiveTest, UserGroupByName())
	register.Register(register.PositiveTest, ForceFileCreation())
	register.Register(register.PositiveTest, ForceFileCreationNoOverwrite())
	register.Register(register.PositiveTest, AppendToAFile())
	register.Register(register.PositiveTest, AppendToNonexistentFile())
	// TODO: Investigate why ignition's C code hates our environment
	// register.Register(register.PositiveTest, UserGroupByName())
}

func CreateFileOnRoot() types.Test {
	name := "Create Files on the Root Filesystem"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": { "source": "data:,example%20file%0A" }
	    }]
	  }
	}`
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
			},
			Contents: "example file\n",
		},
	})
	configMinVersion := "2.0.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func UserGroupByID() types.Test {
	name := "User/Group by id"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": { "source": "data:,example%20file%0A" },
		  "user": {"id": 500},
		  "group": {"id": 500}
	    }]
	  }
	}`
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
				User:      500,
				Group:     500,
			},
			Contents: "example file\n",
		},
	})
	configMinVersion := "2.0.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func UserGroupByName() types.Test {
	name := "User/Group by name"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	in[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "passwd",
				Directory: "etc",
			},
			Contents: "root:x:0:0:root:/root:/bin/bash\ncore:x:500:500:CoreOS Admin:/home/core:/bin/bash\n",
		},
		{
			Node: types.Node{
				Name:      "group",
				Directory: "etc",
			},
			Contents: "root:x:0:root\nwheel:x:10:root,core\n",
		},
	})
	mntDevices := []types.MntDevice{
		{
			Label:        "OEM",
			Substitution: "$DEVICE",
		},
	}
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "filesystems": [{
	      "name": "oem",
	      "mount": {
	        "device": "$DEVICE",
		"format": "ext4"
	      }
	    }],
	    "files": [{
	      "filesystem": "oem",
	      "path": "/foo/bar",
	      "contents": { "source": "data:,example%20file%0A" },
		  "user": {"name": "core"},
		  "group": {"name": "wheel"}
	    }]
	  }
	}`
	out[0].Partitions.AddFiles("OEM", []types.File{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
				User:      500,
				Group:     10,
			},
			Contents: "example file\n",
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
		MntDevices:       mntDevices,
	}
}

func ForceFileCreation() types.Test {
	name := "Force File Creation"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": {
	        "source": "http://127.0.0.1:8080/contents"
	      },
		  "overwrite": true
	    }]
	  }
	}`
	in[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Contents: "hello, world",
		},
	})
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Contents: "asdf\nfdsa",
		},
	})
	configMinVersion := "2.2.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func ForceFileCreationNoOverwrite() types.Test {
	name := "Force File Creation No Overwrite"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": {
	        "source": "http://127.0.0.1:8080/contents"
	      }
	    }]
	  }
	}`
	in[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Contents: "hello, world",
		},
	})
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Contents: "asdf\nfdsa",
		},
	})
	configMinVersion := "2.0.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func AppendToAFile() types.Test {
	name := "Append to a file"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": { "source": "data:,example%20file%0A" },
	      "user": {"id": 500},
	      "group": {"id": 500}
	    },{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": { "source": "data:,hello%20world%0A" },
	      "group": {"id": 0},
	      "append": true
	    }]
	  }
	}`
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
				User:      500,
				Group:     0,
			},
			Contents: "example file\nhello world\n",
		},
	})
	configMinVersion := "2.2.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func AppendToNonexistentFile() types.Test {
	name := "Append to a non-existent file"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "contents": { "source": "data:,hello%20world%0A" },
	      "group": {"id": 500},
	      "append": true
	    }]
	  }
	}`
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
				Group:     500,
			},
			Contents: "hello world\n",
		},
	})
	configMinVersion := "2.2.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}
