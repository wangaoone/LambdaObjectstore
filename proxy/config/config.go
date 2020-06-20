package config

import (
	"time"
)

// AWSRegion Region of AWS services.
const AWSRegion = "us-east-1"

// LambdaMaxDeployments Number of Lambda function deployments available.
const LambdaMaxDeployments = 400

// NumLambdaClusters Number of Lambda function deployments initiated on launching.
const NumLambdaClusters = 100

// LambdaStoreName Obsoleted. Name of Lambda function for replica version.
const LambdaStoreName = "LambdaStore"

// LambdaPrefix Prefix of Lambda function.
const LambdaPrefix = "CacheNode"

// InstanceWarmTimout Interval to warmup Lambda functions.
const InstanceWarmTimout = 1 * time.Minute

// InstanceCapacity Capacity of deployed Lambda functions.
// TODO: Detectable on invocation. Can be specified by option -funcap for now.
const InstanceCapacity = 2048 * 1000000    // MB

// InstanceOverhead Memory reserved for running program on Lambda functions.
const InstanceOverhead = 400 * 1000000     // MB

// ServerPublicIp Public IP of proxy, leave empty if running Lambda functions in VPC.
const ServerPublicIp = ""                  // Leave it empty if using VPC.

// RecoverRate Empirical S3 download rate for specified InstanceCapacity.
// 40MB for 512, 1024, 1536MB instance, 70MB for 3008MB instance.
const RecoverRate = 100 * 1000000           // Not actually used.

// BackupsPerInstance  Number of backup instances used for parallel recovery.
const BackupsPerInstance = 20              // (InstanceCapacity - InstanceOverhead) / RecoverRate

// ProxyList Ip addresses of proxies.
// private ip addr and ports for all proxies if multiple proxies are needed
// If running on one proxy, then can be left empty. But for multiple, build static proxy list here
// of private ip addr. and port.
//var ProxyList []string
var ProxyList [2]string = [2]string{"10.0.119.246:6378", "10.0.101.76:6378"}
