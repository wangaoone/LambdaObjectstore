package server

import (
	"time"

	"github.com/mason-leap-lab/infinicache/proxy/lambdastore"
)

const AWSRegion = "us-east-1"
const LambdaMaxDeployments = 400
const NumLambdaClusters = 12
const LambdaStoreName = "LambdaStore"      // replica version (no use)
const LambdaPrefix = "Store1VPCNode"
const InstanceWarmTimout = 1 * time.Minute
const InstanceCapacity = 1024 * 1000000    // MB
const InstanceOverhead = 100 * 1000000     // MB
const ServerPublicIp = ""                  // Leave it empty if using VPC.
const RecoverRate = 40 * 1000000           // 40MB for 1536MB instance, 70MB for 3008MB instance.
const BackupsPerInstance = 36              // (InstanceCapacity - InstanceOverhead) / RecoverRate

func init() {
	lambdastore.WarmTimout = InstanceWarmTimout
}
