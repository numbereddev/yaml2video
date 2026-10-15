package render

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// assStep is the sampling interval for animated text. Within each segment libass
// interpolates linearly (\move and \t), so any easing curve is followed closely.
const assStep = 50 * time.Millisecond

// textRuns groups consecutive text clips: each run renders through one ASS
// document at its place in the stack.
func textRuns(clips []ClipPlan) [][]ClipPlan {
	var runs [][]ClipPlan
	var run []ClipPlan
	for _, clip := range clips {
		if clip.Kind == "text" {
			run = append(run, clip)
			continue
		}
		if len(run) > 0 {
			runs = append(runs, run)
			run = nil
		}
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}
	return runs
}

func runASSName(track TrackPlan, run int) string {
	return fmt.Sprintf("%s-text-%02d.ass", track.ASSPrefix, run)
}

// textRunDocument renders a run of text clips, timed on their owner's timeline.
func textRunDocument(video VideoSpec, run []ClipPlan) string {
	var b strings.Builder
	writeV2ASSHeader(&b, video, true)
	for _, clip := range run {
		if clip.Layer.Animation != nil {
			writeAnimatedText(&b, clip.Layer, clip.Layer.Align, clip.Start, clip.Start+clip.Duration)
		} else {
			writeStaticText(&b, clip.Layer, clip.Layer.Align, "Advanced", clip.Start, clip.Start+clip.Duration)
		}
	}
	return b.String()
}

type textState struct {
	x, y, scaleX, scaleY, rotation, opacity float64
}

func writeAnimatedText(b *strings.Builder, layer LayerPlan, align string, start, end time.Duration) {
	alignment, x, y, style, tags, fadeInMS, fadeOutMS, value := v2TextParts(layer, align)
	fadeIn, fadeOut := time.Duration(fadeInMS)*time.Millisecond, time.Duration(fadeOutMS)*time.Millisecond
	outline, shadow := 1.0, 1.0
	for _, fx := range layer.Effects {
		switch fx.Type {
		case "text_glow", "text_outline":
			outline = fx.Opacity
		case "text_shadow":
			shadow = fx.Opacity
		}
	}

	anim, length := layer.Animation, end-start
	sample := func(t time.Duration) textState {
		scale := anim.Value("scale", t, 1)
		opacity := anim.Value("opacity", t, 1)
		// Fades are folded into opacity because \fad would restart on every segment.
		if fadeIn > 0 && t < fadeIn {
			opacity *= float64(t) / float64(fadeIn)
		}
		if fadeOut > 0 && t > length-fadeOut {
			opacity *= float64(length-t) / float64(fadeOut)
		}
		return textState{
			x:        float64(x) + anim.Value("x", t, 0),
			y:        float64(y) + anim.Value("y", t, 0),
			scaleX:   100 * scale * anim.Value("scale_x", t, 1),
			scaleY:   100 * scale * anim.Value("scale_y", t, 1),
			rotation: -anim.Value("rotation", t, 0), // ASS rotates counterclockwise
			opacity:  math.Max(0, math.Min(1, opacity)),
		}
	}
	state := func(s textState) string {
		return fmt.Sprintf("\\fscx%.2f\\fscy%.2f\\frz%.2f\\1a%s\\3a%s\\4a%s",
			s.scaleX, s.scaleY, s.rotation, v2ASSAlpha(s.opacity), v2ASSAlpha(s.opacity*outline), v2ASSAlpha(s.opacity*shadow))
	}

	for _, span := range layer.visible(start, end) {
		for t0 := span.Start - start; t0 < span.End-start; t0 += assStep {
			t1 := min(t0+assStep, span.End-start)
			s0, s1 := sample(t0), sample(t1)
			fmt.Fprintf(b, "Dialogue: 0,%s,%s,Advanced,,0,0,0,,{\\an%d\\move(%.1f,%.1f,%.1f,%.1f)%s%s%s\\t(%s)}%s\n",
				v2ASSTime(start+t0), v2ASSTime(start+t1), alignment, s0.x, s0.y, s1.x, s1.y, style, tags, state(s0), state(s1), value)
		}
	}
}

// visible returns the parts of [start, end) where the layer is shown.
func (l LayerPlan) visible(start, end time.Duration) []Window {
	spans := []Window{{start, end}}
	if l.ShowOnly != nil {
		spans = []Window{{max(start, l.ShowOnly.Start), min(end, l.ShowOnly.End)}}
		if spans[0].End <= spans[0].Start {
			return nil
		}
	}
	for _, hide := range l.HideDuring {
		var kept []Window
		for _, s := range spans {
			if hide.End <= s.Start || hide.Start >= s.End {
				kept = append(kept, s)
				continue
			}
			if hide.Start > s.Start {
				kept = append(kept, Window{s.Start, hide.Start})
			}
			if hide.End < s.End {
				kept = append(kept, Window{hide.End, s.End})
			}
		}
		spans = kept
	}
	return spans
}

// enableExpr is the FFmpeg timeline expression for the layer's windows, or ""
// when it is always shown.
func (l LayerPlan) enableExpr() string {
	var parts []string
	if l.ShowOnly != nil {
		parts = append(parts, fmt.Sprintf("gte(t,%s)*lt(t,%s)", ffTime(l.ShowOnly.Start), ffTime(l.ShowOnly.End)))
	}
	for _, hide := range l.HideDuring {
		parts = append(parts, fmt.Sprintf("(1-gte(t,%s)*lt(t,%s))", ffTime(hide.Start), ffTime(hide.End)))
	}
	return strings.Join(parts, "*")
}
