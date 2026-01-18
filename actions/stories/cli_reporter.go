package stories

import (
	"fmt"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/go-instagram-cli/internal/platform/instagram"
)

type CLIReporter struct {
	progress *mpb.Progress
	master   *mpb.Bar
	mu       sync.Mutex

	statusMsg    string
	bytesHandled int64
}

func NewCLIReporter() *CLIReporter {
	return &CLIReporter{
		progress:  mpb.New(mpb.WithWidth(60)),
		statusMsg: "Initializing...",
	}
}

func (r *CLIReporter) Report(p instagram.ProgressReport) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p.Step == "INIT" && r.master == nil {
		r.master = r.progress.AddBar(p.TotalBytes,
			mpb.PrependDecorators(
				decor.Any(func(st decor.Statistics) string {
					return fmt.Sprintf("%-15s", r.statusMsg)
				}, decor.WCSyncSpaceR),
				decor.Counters(decor.SizeB1024(0), "% .2f / % .2f", decor.WCSyncSpace),
			),
			mpb.AppendDecorators(
				decor.AverageSpeed(decor.SizeB1024(0), "% .2f", decor.WCSyncSpace),
				decor.Name(" | "),
				decor.OnComplete(
					decor.AverageETA(decor.ET_STYLE_GO), "✨ Done!",
				),
			),
		)
		return
	}

	if r.master == nil {
		r.statusMsg = p.Step
		return
	}

	switch p.Step {
	case "UPLOAD":
		r.statusMsg = fmt.Sprintf("🎬 Part %d/%d", p.Current, p.Total)
		r.master.SetCurrent(r.bytesHandled + p.BytesSent)

	case "CONFIG":
		r.statusMsg = fmt.Sprintf("⚙️  Config %d/%d", p.Current, p.Total)
		if p.BytesSent == 0 && p.TotalBytes > 0 {
			r.bytesHandled += p.TotalBytes
			r.master.SetCurrent(r.bytesHandled)
		}

	case "PREPARE":
		r.statusMsg = "📦 Preparing..."
	}
}

func (r *CLIReporter) Wait() {
	r.progress.Wait()
}
