package global

import (
	"github.com/cornelk/hashmap"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"sync"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
)

var (
	// Clients        = make([]chan interface{}, 1024*1024)
	DataCollected    sync.WaitGroup
	Log              logger.ILogger
	ReqMap           = hashmap.New(1024)
	Migrator         types.MigrationScheduler
	BasePort         = 6378
	BaseMigratorPort = 6380
	ServerIp         string
	Prefix           string
	Vpc              bool
)

func init() {
	Log = logger.NilLogger

	//ip, err := GetPrivateIp()
	//ip, err := GetIP(Vpc)
	//if err != nil {
	//	panic(err)
	//}

	ServerIp = "3.228.28.69"
}
