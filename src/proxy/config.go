package proxy

import (
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const LambdaMaxDeployments = 10
const NumLambdaClusters = 10
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy2Node"
const InstanceWarmTimout = 10 * time.Minute
const InstanceCapacity = 200 * 1000000    // MB
const InstanceOverhead = 100 * 1000000     // MB

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
