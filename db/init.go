// Copyright 2014 The Cayley Authors. All rights reserved.
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

package db

import (
	"github.com/google/cayley/config"
	"github.com/google/cayley/graph/leveldb"
	"github.com/google/cayley/graph/mongo"
)

func Init(cfg *config.Config, triplePath string) bool {
	created := false
	dbpath := cfg.DatabasePath
	switch cfg.DatabaseType {
	case "mongo", "mongodb":
		created = mongo.CreateNewMongoGraph(dbpath, cfg.DatabaseOptions)
	case "leveldb":
		created = leveldb.CreateNewLevelDB(dbpath)
	case "mem":
		return true
	}
	if created && triplePath != "" {
		ts := Open(cfg)
		Load(ts, cfg, triplePath, true)
		ts.Close()
	}
	return created
}
