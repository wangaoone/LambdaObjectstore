package config

import (
	"time"
)

// LambdaPrefix Prefix of Lambda function, overridable with command line parameter -lambda-prefix.
const LambdaPrefix = "CacheNode0-"

// AWSRegion Region of AWS services.
const AWSRegion = "us-east-1"

// LambdaMaxDeployments Number of Lambda function deployments available.
const LambdaMaxDeployments = 300

// Mode of cluster.
const StaticCluster = "static"
const WindowCluster = "window"
const Cluster = WindowCluster

// Size of a slice if the cluster implementation support. Client library use this value to initialize chunk placements.
const SliceSize = 100

// NumLambdaClusters Number of Lambda function deployments initiated on launching.
// For window cluster, this must be at least D+P
const NumLambdaClusters = 12

// LambdaStoreName Obsoleted. Name of Lambda function for replica version.
const LambdaStoreName = "LambdaStore"

// InstanceWarmTimout Interval to warmup Lambda functions.
const InstanceWarmTimeout = 1 * time.Minute

// Instance degrade warmup interval
const InstanceDegradeWarmTimeout = 5 * time.Minute

// InstanceCapacity Capacity of deployed Lambda functions.
// TODO: Detectable on invocation. Can be specified by option -funcap for now.
const DefaultInstanceCapacity = 1024 * 1000000 // 1GB

// InstanceOverhead Memory reserved for running program on Lambda functions.
const InstanceOverhead = 300 * 1000000 // 300 MB

// Threshold Scaling out avg instance size threshold
const Threshold = 0.8 // Don't set beyond 0.8

// Maximum chunk per instance
const ChunkThreshold = 125000 // Fraction, ChunkThreshold = InstanceCapacity / 100K * Threshold

// ServerPublicIp Public IP of proxy, leave empty if running Lambda functions in VPC.
const ServerPublicIp = "" // Leave it empty if Lambda VPC is enabled.

// RecoverRate Empirical S3 download rate for specified InstanceCapacity.
// 40MB for 512, 1024, 1536MB instance, 70MB for 3008MB instance.
const RecoverRate = 40 * 1000000 // Not actually used.

// BackupsPerInstance  Number of backup instances used for parallel recovery.
const BackupsPerInstance = 20 // (InstanceCapacity - InstanceOverhead) / RecoverRate

// Each bucket's active duration
const BucketDuration = 1 // min

// Number of buckets that warmup every InstanceWarmTimeout
const NumActiveBuckets = 10 // Keep data in cache with max availability for 10 mins

// Number of buckets before expiring
// Buckets beyond NumActiveBuckets but within ExpireBucketsNum will get degraded warmup: InstanceDegradeWarmTimeout
const NumAvailableBuckets = 30 // Keep data in cache with best effort for 30 mins

// Async migrate control
const ActiveReplica = 2 //min

// ProxyList Ip addresses of proxies.
// private ip addr and ports for all proxies if multiple proxies are needed
// If running on one proxy, then can be left empty. But for multiple, build static proxy list here
// of private ip addr. and port.
//var ProxyList []string
var ProxyList []string = make([]string, 0) // []string{"10.0.119.246:6378", "10.0.113.107:6378"}
//var ProxyList []string = []string{"10.0.109.88:6378", "10.0.106.200:6378", "10.0.120.129:6378", "10.0.118.74:6378"}
