package server

import (
	"sync/atomic"
	"time"

	"github.com/mason-leap-lab/infinicache/common/logger"
	"github.com/mason-leap-lab/infinicache/proxy/global"
)

// reuse window and interval should be MINUTES
type MovingWindow struct {
	proxy    *Proxy
	log      logger.ILogger
	window   int
	interval int
	num      int // number of hot bucket 1 hour time window = 6 num * 10 min
	buckets  []*bucket

	cursor    int
	startTime time.Time

	scaler       chan struct{}
	scaleCounter int32
}

func NewMovingWindow(window int, interval int) *MovingWindow {
	// number of active time bucket
	//num := window / interval
	return &MovingWindow{
		log: &logger.ColorLogger{
			Prefix: "Moving window ",
			Level:  global.Log.GetLevel(),
			Color:  true,
		},
		window:    window,
		interval:  interval,
		buckets:   make([]*bucket, 0, 9999),
		startTime: time.Now(),
		cursor:    0,

		// for scaling out
		scaler:       make(chan struct{}, 1),
		scaleCounter: 0,
	}
}

func (mw *MovingWindow) waitReady() {
	mw.getCurrentBucket().waitReady()
}

// only assign backup for new node in bucket
func (mw *MovingWindow) assignBackup(bucket *bucket) {
	length := len(bucket.group.All)

	for i := length - NumLambdaClusters; i < length; i++ {
		num, candidates := scheduler.getBackupsForNode(bucket.group, i)
		node := mw.proxy.group.Instance(int(i))
		node.AssignBackups(num, candidates)
	}
}

func (mw *MovingWindow) start() *Group {
	// init bucket
	bucket := newBucket(0)

	// assign backup node for all nodes of this bucket
	mw.assignBackup(bucket)

	// append to bucket list & append current bucket group to proxy group
	mw.appendToGroup(bucket.group)
	mw.buckets = append(mw.buckets, bucket)

	return bucket.group
}

func (mw *MovingWindow) Daemon() {
	idx := 1
	for {
		//ticker := time.NewTicker(time.Duration(mw.interval) * time.Minute)
		ticker := time.NewTicker(30 * time.Second)
		select {
		// scaling out in bucket
		case <-mw.scaler:

			// get current bucket
			bucket := mw.getCurrentBucket()

			tmpGroup := NewGroup(NumLambdaClusters)
			for i := range tmpGroup.All {
				node := scheduler.GetForGroup(tmpGroup, i)
				node.Meta.Capacity = InstanceCapacity
				node.Meta.IncreaseSize(InstanceOverhead)
				go func() {
					node.WarmUp()
					if atomic.AddInt32(&mw.scaleCounter, 1) == int32(len(tmpGroup.All)) {
						mw.log.Info("[scale out is ready]")
					}
				}()
				// Begin handle requests
				go node.HandleRequests()
			}

			mw.assignBackup(bucket)

			// reset counter
			mw.scaleCounter = 0

			// append tmpGroup to current bucket group
			bucket.append(tmpGroup)
			bucket.rang += NumLambdaClusters

			// append tmnGroup to proxy group
			mw.appendToGroup(tmpGroup)

			// move pointer after scale out
			atomic.AddInt32(&mw.proxy.placer.pointer, NumLambdaClusters)

			//scale out phase finished
			mw.proxy.placer.scaling = false
			mw.log.Debug("scale out finish")

		// for bucket rolling
		case <-ticker.C:
			//TODO: generate new fake bucket. use the same pointer as last bucket
			//currentBucket := mw.getCurrentBucket()
			//if mw.avgSize(currentBucket) < 1000 {
			//	break
			//}

			bucket := newBucket(idx)
			mw.assignBackup(bucket)

			// append to bucket list & append current bucket group to proxy group
			mw.appendToGroup(bucket.group)
			mw.buckets = append(mw.buckets, bucket)

			// increase proxy group pointer and sync bucket start index
			bucket.start = atomic.AddInt32(&mw.proxy.placer.pointer, NumLambdaClusters)
			mw.cursor = len(mw.buckets) - 1

			mw.log.Debug("current placer from is %v, step is %v", atomic.LoadInt32(&mw.proxy.placer.pointer), NumLambdaClusters)
		}
		idx += 1
	}
}

// retrieve cold bucket (first half)
func (mw *MovingWindow) getColdBucket() []*bucket {
	if len(mw.buckets) <= mw.num {
		return nil
	} else {
		return mw.buckets[0 : len(mw.buckets)/2-1]
	}
}

// retrieve hot bucket (second half)
func (mw *MovingWindow) getActiveBucket() []*bucket {
	if len(mw.buckets) <= mw.num {
		return mw.buckets
	} else {
		return mw.buckets[len(mw.buckets)/2:]
	}
}

func (mw *MovingWindow) getAllBuckets() []*bucket {
	return mw.buckets
}

func (mw *MovingWindow) getCurrentBucket() *bucket {
	return mw.buckets[len(mw.buckets)-1]
}

// active group means active bucket under N(2) hour window
//func (mw *MovingWindow) getActiveGroup() *Group {
//	res := &Group{
//		All:  make([]*GroupInstance, 0, LambdaMaxDeployments),
//		size: 0,
//	}
//	for _, bucket := range mw.getActiveBucket() {
//		g := bucket.group
//		for i := 0; i < g.Len(); i++ {
//			res.All = append(res.All, g.All[i])
//		}
//	}
//	res.size = len(res.All)
//	return res
//}

//func (mw *MovingWindow) getAllGroup() *Group {
//	res := &Group{
//		All:  make([]*GroupInstance, 0, LambdaMaxDeployments),
//		size: 0,
//	}
//	for _, bucket := range mw.getAllBuckets() {
//		g := bucket.group
//		mw.log.Debug("bucket id is %v", bucket.id)
//		for i := 0; i < len(g.All); i++ {
//			mw.log.Debug("active instance name %v", g.All[i].Name())
//			res.All = append(res.All, g.All[i])
//		}
//	}
//	res.size = len(res.All)
//	return res
//}

func (mw *MovingWindow) appendToGroup(g *Group) {
	for i := 0; i < len(g.All); i++ {
		mw.proxy.group.All = append(mw.proxy.group.All, g.All[i])
		mw.log.Debug("active instance name %v", g.All[i].Name())
	}
}
func (mw *MovingWindow) getInstanceId(id int, from int) int {
	//idx := mw.getCurrentBucket().from + id
	idx := id + from
	return idx
}

func (mw *MovingWindow) touch(meta *Meta) {
	mw.log.Debug("in touch %v", meta.Placement)
	// brand new meta(-1) or already existed
	if meta.placerMeta.bucketIdx == -1 {
		mw.buckets[mw.cursor].m.Set(meta.Key, meta)
	} else {
		// remove meta from previous bucket
		oldBucket := meta.placerMeta.bucketIdx
		if mw.cursor == oldBucket {
			return
		} else {
			mw.buckets[oldBucket].m.Del(meta.Key)
			mw.buckets[mw.cursor].m.Set(meta.Key, meta)
		}
	}

	meta.placerMeta.bucketIdx = mw.cursor
	//meta.placerMeta.ts = time.Now().UnixNano()
}

func (mw *MovingWindow) avgSize(bucket *bucket) int {
	sum := 0
	start := bucket.start

	for i := start; i < start+bucket.rang; i++ {
		sum += int(mw.proxy.group.Instance(int(i)).Meta.Size())
	}

	return sum / int(bucket.rang)
}

//func (mw *MovingWindow) updateBucket(meta *Meta, lastChunk int) {
//	oldBucket := meta.placerMeta.bucketIdx
//
//	if oldBucket == -1 {
//		mw.log.Debug("Not found in moving window, please set it first")
//		return
//	}
//
//	if mw.cursor == oldBucket {
//		return
//	} else {
//		mw.buckets[oldBucket].m.Del(meta.Key)
//		mw.buckets[mw.cursor].m.Set(meta.Key, meta)
//		meta.placerMeta.bucketIdx = mw.cursor
//	}
//	meta.placerMeta.ts = time.Now().UnixNano()
//
//}

//func (mw *MovingWindow) cursorMove() {
//	new := mw.cursor - 1
//	if new < 0 {
//		mw.cursor = len(mw.buckets) - 1
//	}
//	mw.cursor = new
//}

// last bucket is @ the left hand of the current cursor
//func (mw *MovingWindow) getLastBucket() int {
//	// All buckets are not full, first bucket is the oldest
//	if int(time.Now().Sub(mw.startTime).Minutes()) <= mw.window {
//		return 0
//	}
//	idx := mw.cursor - 1
//	if idx < 0 {
//		mw.cursor = len(mw.buckets) - 1
//	}
//	return idx
//}

//func (mw *MovingWindow) removeOldest(bucketIdx int) (*Meta, bool) {
//	if mw.buckets[bucketIdx].size() == 0 {
//		return nil, false
//	}
//	var res *Meta
//	bucket := mw.buckets[bucketIdx]
//
//	for i := range bucket.m.Iter() {
//		ts := i.Value.(*Meta).placerMeta.ts
//		if ts < oldest {
//			oldest = ts
//			res = i.Value.(*Meta)
//		}
//	}
//	mw.buckets[bucketIdx].m.Del(res.Key)
//	return res, true
//}

func (mw *MovingWindow) evict(idx int) {
	// TODO: all the metas in last bucket need to be evicted
}

//func (mw *MovingWindow) Check() {
//	for {
//		t := time.NewTicker(time.Duration(mw.interval) * time.Minute)
//		select {
//		case <-t.C:
//			mw.evict(mw.getLastBucket())
//			mw.cursorMove()
//		}
//	}
//}

//func (mw *MovingWindow) Start() {
//	mw.start = time.Now()
//}
//
//func (mw *MovingWindow) getBucket(meta *Meta) int {
//	delta := meta.placerMeta.currentTS.Sub(mw.start).Minutes()
//	windowIdx := int(delta)/mw.interval + 1
//	//return int(math.Mod(delta, float64(mw.interval)))
//	return windowIdx
//}
//
//func (mw *MovingWindow) Add(meta *Meta) bool {
//	// get bucket idx
//	meta.movingMeta.bucketIdx = mw.getBucket(meta)
//	_, loaded := mw.buckets[meta.movingMeta.bucketIdx].GetOrInsert(meta.Key, meta)
//
//	if !loaded {
//		mw.log.Debug("Already stored")
//		return loaded
//	}
//
//	return loaded
//}
//
//func (mw *MovingWindow) Update(meta *Meta) {
//	delta := int(meta.placerMeta.reuseTime.Minutes())
//	if delta < mw.reuseDistance {
//
//		newIdx := mw.getBucket(meta)
//		lastIdx := meta.movingMeta.bucketIdx
//
//		// remove previous bucket meta
//		mw.buckets[lastIdx].Del(meta.Key)
//		mw.buckets[newIdx].Set(meta.Key, meta)
//	}
//}

// each bucket check and move the validate meta
//func (mw *MovingWindow) bucketCheck(idx int, wg *sync.WaitGroup) {
//	bucket := mw.buckets[idx]
//	for i := range bucket.Iter() {
//		meta := i.Value.(*Meta)
//
//		// never be touched after first insert
//		if meta.placerMeta.lastTS.IsZero() {
//			meta.placerMeta.reuseTime = time.Now().Sub(meta.placerMeta.currentTS)
//		} else {
//			// update meta reuse distance
//			meta.placerMeta.reuseTime += CheckInterval
//		}
//		if int(meta.placerMeta.reuseTime.Minutes()) > mw.window {
//			// timeout, del key and move to evict list
//			bucket.Del(i.Key)
//			mw.window = append(mw.evictQueue, meta)
//		} else {
//			// TODO: move to other bucket idx
//			newIdx := mw.findBucket(meta)
//
//			// move to other corresponding bucket
//			if newIdx != idx {
//				bucket.Del(i.Key)
//				mw.buckets[newIdx].Set(i.Key, meta)
//			}
//
//		}
//
//	}
//	wg.Done()
//
//}
