package proxy

import (
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const LambdaMaxDeployments = 10
const NumLambdaClusters = 10
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy2Node"
const InstanceWarmTimout = 1 * time.Minute

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
