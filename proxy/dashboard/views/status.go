package views

import (
	"fmt"
	"image"
	"runtime"

	"github.com/dustin/go-humanize"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/mason-leap-lab/infinicache/common/util"
	"github.com/mason-leap-lab/infinicache/proxy/global"
	"github.com/mason-leap-lab/infinicache/proxy/types"
)

var (
	MemLimit = uint64(0)
)

type StatusView struct {
	*widgets.Paragraph
	Meta types.MetaStoreStats

	dash        DashControl
	serverStats types.ServerStats
	memStat     runtime.MemStats
	maxMemory   uint64
}

func NewStatusView(dash DashControl) *StatusView {
	view := &StatusView{
		Paragraph: widgets.NewParagraph(),
		dash:      dash,
	}
	view.serverStats = view
	view.Border = false
	return view
}

// SetServerStats provides the view with server stats.
func (v *StatusView) SetServerStats(svrStats types.ServerStats) {
	v.serverStats = svrStats
}

func (v *StatusView) Draw(buf *ui.Buffer) {
	runtime.ReadMemStats(&v.memStat)
	mem := v.memStat.Sys - v.memStat.HeapSys - v.memStat.GCSys + v.memStat.HeapInuse
	if v.maxMemory < mem {
		v.maxMemory = mem
	}
	v.Text = fmt.Sprintf("Show occupancy(m): %v, Mem: %s, Max: %s, Objects: %d, Serving: %d, PCached: %d",
		v.dash.GetOccupancyMode(), humanize.Bytes(mem), humanize.Bytes(v.maxMemory),
		util.Ifelse(v.Meta != nil, v.Meta.Len(), 0).(int), global.ReqCoordinator.Len(), v.serverStats.PersistCacheLen())

	// Draw overwrite
	v.Block.Draw(buf)

	cells := ui.ParseStyles(v.Text, v.TextStyle)
	if v.WrapText {
		cells = ui.WrapCells(cells, uint(v.Inner.Dx()))
	}

	for _, cx := range ui.BuildCellWithXArray(cells) {
		x, cell := cx.X, cx.Cell
		buf.SetCell(cell, image.Pt(x, v.Inner.Max.Y-v.Inner.Min.Y-1).Add(v.Inner.Min))
	}

	if MemLimit > 0 && v.maxMemory > MemLimit {
		go func(dash DashControl) {
			runtime.Gosched()
			dash.Quit(fmt.Sprintf("Memory OOM alert: HeapAlloc beyond %s(%s)", humanize.Bytes(MemLimit), humanize.Bytes(v.maxMemory)))
		}(v.dash)
	}
}

// PersistCacheLen implements dummy types.ServerStats interface.
func (v *StatusView) PersistCacheLen() int {
	return 0
}
