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
	register.Register(register.PositiveTest, CreateHardLinkOnRoot())
	register.Register(register.PositiveTest, CreateSymlinkOnRoot())
	register.Register(register.PositiveTest, WriteThroughRelativeSymlink())
	register.Register(register.PositiveTest, WriteThroughRelativeSymlinkBeyondRoot())
	register.Register(register.PositiveTest, WriteThroughAbsoluteSymlink())
	register.Register(register.PositiveTest, ForceLinkCreation())
	register.Register(register.PositiveTest, ForceHardLinkCreation())
	register.Register(register.PositiveTest, WriteOverSymlink())
	register.Register(register.PositiveTest, WriteOverBrokenSymlink())
}

func CreateHardLinkOnRoot() types.Test {
	name := "Create a Hard Link on the Root Filesystem"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/target",
	      "contents": {
	        "source": "http://127.0.0.1:8080/contents"
	      }
	    }],
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
		  "target": "/foo/target",
		  "hard": true
	    }]
	  }
	}`
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "target",
			},
			Contents: "asdf\nfdsa",
		},
	})
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Target: "/foo/target",
			Hard:   true,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func CreateSymlinkOnRoot() types.Test {
	name := "Create a Symlink on the Root Filesystem"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "target": "/foo/target",
	      "hard": false
	    }]
	  }
	}`
	in[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "target",
				Directory: "foo",
			},
		},
	})
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
			},
			Target: "/foo/target",
			Hard:   false,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func WriteThroughRelativeSymlink() types.Test {
	name := "Write Through Relative Symlink on the Root Filesystem"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	// note this abuses the order in which ignition writes links and will break with 3.0.0
	// Also tests that Ignition does not try to resolve symlink targets
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "target": "../etc"
	    },
	    {
	      "filesystem": "root",
	      "path": "/foo/bar/baz",
	      "target": "somewhere/over/the/rainbow"
	    }]
	  }
	}`
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
			},
			Target: "../etc",
			Hard:   false,
		},
		{
			Node: types.Node{
				Name:      "baz",
				Directory: "etc",
			},
			Target: "somewhere/over/the/rainbow",
			Hard:   false,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func WriteThroughRelativeSymlinkBeyondRoot() types.Test {
	name := "Write Through Relative Symlink beyond the root on the Root Filesystem"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	// note this abuses the order in which ignition writes links and will break with 3.0.0
	// Also tests that Ignition does not try to resolve symlink targets
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "target": "../../etc"
	    },
	    {
	      "filesystem": "root",
	      "path": "/foo/bar/baz",
	      "target": "somewhere/over/the/rainbow"
	    }]
	  }
	}`
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
			},
			Target: "../../etc",
			Hard:   false,
		},
		{
			Node: types.Node{
				Name:      "baz",
				Directory: "etc",
			},
			Target: "somewhere/over/the/rainbow",
			Hard:   false,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func WriteThroughAbsoluteSymlink() types.Test {
	name := "Write Through Absolute Symlink on the Root Filesystem"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	// note this abuses the order in which ignition writes links and will break with 3.0.0
	// Also tests that Ignition does not try to resolve symlink targets
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "target": "/etc"
	    },
	    {
	      "filesystem": "root",
	      "path": "/foo/bar/baz",
	      "target": "somewhere/over/the/rainbow"
	    }]
	  }
	}`
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Name:      "bar",
				Directory: "foo",
			},
			Target: "/etc",
			Hard:   false,
		},
		{
			Node: types.Node{
				Name:      "baz",
				Directory: "etc",
			},
			Target: "somewhere/over/the/rainbow",
			Hard:   false,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func ForceLinkCreation() types.Test {
	name := "Force Link Creation"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/target",
	      "contents": {
	        "source": "http://127.0.0.1:8080/contents"
	      }
	    }],
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "target": "/foo/target",
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
			Contents: "asdf\nfdsa",
		},
	})
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "target",
			},
			Contents: "asdf\nfdsa",
		},
	})
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Target: "/foo/target",
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

func ForceHardLinkCreation() types.Test {
	name := "Force Hard Link Creation"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/foo/target",
	      "contents": {
	        "source": "http://127.0.0.1:8080/contents"
	      }
	    }],
	    "links": [{
	      "filesystem": "root",
	      "path": "/foo/bar",
	      "target": "/foo/target",
		  "hard": true,
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
			Contents: "asdf\nfdsa",
		},
	})
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "target",
			},
			Contents: "asdf\nfdsa",
		},
	})
	out[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Directory: "foo",
				Name:      "bar",
			},
			Target: "/foo/target",
			Hard:   true,
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

func WriteOverSymlink() types.Test {
	name := "Write Over Symlink at end of path"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	// note this abuses the order in which ignition writes links and will break with 3.0.0
	// Also tests that Ignition does not try to resolve symlink targets
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/etc/file",
	      "mode": 420
	    }]
	  }
	}`
	in[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Name:      "file",
				Directory: "etc",
			},
			Target: "/usr/rofile",
		},
	})
	in[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "rofile",
				Directory: "usr",
			},
			Contents: "",
			Mode:     420,
		},
	})
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "rofile",
				Directory: "usr",
			},
			Contents: "",
			Mode:     420,
		},
		{
			Node: types.Node{
				Name:      "file",
				Directory: "etc",
			},
			Contents: "",
			Mode:     420,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}

func WriteOverBrokenSymlink() types.Test {
	name := "Write Over Broken Symlink at end of path"
	in := types.GetBaseDisk()
	out := types.GetBaseDisk()
	// note this abuses the order in which ignition writes links and will break with 3.0.0
	// Also tests that Ignition does not try to resolve symlink targets
	config := `{
	  "ignition": { "version": "$version" },
	  "storage": {
	    "files": [{
	      "filesystem": "root",
	      "path": "/etc/file",
	      "mode": 420
	    }]
	  }
	}`
	in[0].Partitions.AddLinks("ROOT", []types.Link{
		{
			Node: types.Node{
				Name:      "file",
				Directory: "etc",
			},
			Target: "/usr/rofile",
		},
	})
	out[0].Partitions.AddFiles("ROOT", []types.File{
		{
			Node: types.Node{
				Name:      "file",
				Directory: "etc",
			},
			Contents: "",
			Mode:     420,
		},
	})
	configMinVersion := "2.1.0"

	return types.Test{
		Name:             name,
		In:               in,
		Out:              out,
		Config:           config,
		ConfigMinVersion: configMinVersion,
	}
}
