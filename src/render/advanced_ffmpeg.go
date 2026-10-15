package render

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// animationMargin pads transform canvases so FFmpeg's edge clamping in
// perspective only ever repeats transparent pixels.
const animationMargin = 8

var cameraProps = []string{"x", "y", "scale", "scale_x", "scale_y", "rotation"}

// applyTracks composites tracks, and each track's clips, in order. Clip times
// are on the stream's own timeline (a scene, or the final video). A run of
// consecutive text clips renders through one ASS document at its position.
func applyTracks(base *ffmpeg.Stream, plan *Plan, tracks []TrackPlan) *ffmpeg.Stream {
	ordinal := 0
	for _, track := range tracks {
		run := 0
		for i := 0; i < len(track.Clips); i++ {
			clip := track.Clips[i]
			switch clip.Kind {
			case "image", "rect", "circle":
				ordinal++
				base = overlayClip(base, plan, clip, ordinal)
			case "box":
				base = base.Filter("drawbox", nil, ffmpeg.KwArgs{
					"x": clip.Layer.X, "y": clip.Layer.Y, "w": clip.Layer.Width, "h": clip.Layer.Height,
					"color": withOpacity(clip.Layer.Color, clip.Layer.Opacity), "t": "fill", "enable": clipEnable(clip),
				})
			case "effect":
				base = applyEffectClip(base, plan, clip)
			case "text":
				for ; i < len(track.Clips) && track.Clips[i].Kind == "text"; i++ {
					base = textClipPanel(base, track.Clips[i])
				}
				i--
				base = base.Filter(ffmpegFilterASS, nil, ffmpeg.KwArgs{"filename": filepath.Join(plan.WorkDir, "ass", runASSName(track, run))})
				run++
			}
		}
	}
	return base
}

// clipEnable is the timeline expression for a clip's window and the layer's
// own show/hide windows.
func clipEnable(clip ClipPlan) string {
	enable := clipWindow(clip)
	if extra := clip.Layer.enableExpr(); extra != "" {
		enable += "*" + extra
	}
	return enable
}

// overlayClip composites one visual clip. ffmpeg-go merges graph nodes with
// identical content, so each clip's source runs a distinct number of
// microseconds longer (by its ordinal); the overlay ends it on time regardless.
func overlayClip(base *ffmpeg.Stream, plan *Plan, clip ClipPlan, ordinal int) *ffmpeg.Stream {
	layer := clip.Layer
	length := clip.Duration + time.Duration(ordinal)*time.Microsecond
	var input *ffmpeg.Stream
	if clip.Kind == "image" {
		input = mediaLayerStream(plan, ScenePlan{Duration: length}, layer)
	} else {
		input = shapeStream(plan, layer, length)
	}

	input, x, y := animateMedia(input, plan, layer, clip.Start)
	if clip.Start > 0 {
		input = input.Filter("setpts", ffmpeg.Args{fmt.Sprintf("PTS+%s/TB", ffTime(clip.Start))})
	}
	args := ffmpeg.KwArgs{"x": x, "y": y}
	if enable := layer.enableExpr(); enable != "" {
		args["enable"] = enable
	}
	return base.Overlay(input, eofActionPass, args)
}

func shapeStream(plan *Plan, layer LayerPlan, d time.Duration) *ffmpeg.Stream {
	source := fmt.Sprintf("color=c=%s:s=%dx%d:r=%d:d=%s", layer.Color, layer.Width, layer.Height, plan.Video.FPS, ffTime(d))
	input := ffmpeg.Input(source, ffmpeg.KwArgs{"f": "lavfi"}).Filter("format", ffmpeg.Args{"rgba"})
	if layer.Opacity != 1 {
		input = input.ColorChannelMixer(ffmpeg.KwArgs{"aa": layer.Opacity})
	}
	if layer.Shape == "circle" || layer.Shape == "rounded-rectangle" {
		input = maskAlpha(input, shapeMask(layer))
	}
	return input
}

// animateMedia applies keyframes to a prepared media stream whose own timeline
// starts at the clip start. It returns the stream and its overlay position;
// start converts the main stream's time t into clip time for x/y expressions.
func animateMedia(input *ffmpeg.Stream, plan *Plan, layer LayerPlan, start time.Duration) (*ffmpeg.Stream, any, any) {
	anim := layer.Animation
	if anim == nil {
		return input, layer.X, layer.Y
	}
	if anim.Has("opacity") {
		input = input.Filter("format", ffmpeg.Args{"gbrap"}).Filter("geq", nil, ffmpeg.KwArgs{
			"r": "r(X,Y)", "g": "g(X,Y)", "b": "b(X,Y)",
			"a": fmt.Sprintf("alpha(X,Y)*clip(%s,0,1)", anim.Expr("opacity", "T", 1)),
		})
	}
	if !anim.Has("scale", "scale_x", "scale_y", "rotation") {
		if !anim.Has("x", "y") {
			return input, layer.X, layer.Y
		}
		t := fmt.Sprintf("(t-%s)", ffTime(start))
		return input, fmt.Sprintf("%d+%s", layer.X, anim.Expr("x", t, 0)), fmt.Sprintf("%d+%s", layer.Y, anim.Expr("y", t, 0))
	}

	// Scale and rotation: place the layer on a transparent canvas that covers
	// the frame and the layer, then move the canvas corners with perspective.
	w, h := plan.Video.Width, plan.Video.Height
	left := min(0, layer.X) - animationMargin
	top := min(0, layer.Y) - animationMargin
	cw := max(w, layer.X+layer.Width) + animationMargin - left
	ch := max(h, layer.Y+layer.Height) + animationMargin - top
	ox, oy := layer.X-left, layer.Y-top

	t := fmt.Sprintf("(in/%d)", plan.Video.FPS)
	input = input.Filter("format", ffmpeg.Args{"yuva444p"}).
		Filter("pad", nil, ffmpeg.KwArgs{"w": cw, "h": ch, "x": ox, "y": oy, "color": "black@0"}).
		Filter("perspective", nil, transformCorners(anim, t, float64(cw), float64(ch), float64(ox)+float64(layer.Width)/2, float64(oy)+float64(layer.Height)/2))
	return input, left, top
}

// transformCorners maps the corners of a cw×ch canvas through translate, scale,
// and clockwise rotation around (px, py), as perspective destination points.
func transformCorners(anim *Animation, t string, cw, ch, px, py float64) ffmpeg.KwArgs {
	sx := fmt.Sprintf("max(%s*%s,0.001)", anim.Expr("scale", t, 1), anim.Expr("scale_x", t, 1))
	sy := fmt.Sprintf("max(%s*%s,0.001)", anim.Expr("scale", t, 1), anim.Expr("scale_y", t, 1))
	r := fmt.Sprintf("(%s*PI/180)", anim.Expr("rotation", t, 0))
	cx := fmt.Sprintf("(%s+%s)", num6(px), anim.Expr("x", t, 0))
	cy := fmt.Sprintf("(%s+%s)", num6(py), anim.Expr("y", t, 0))

	args := ffmpeg.KwArgs{"sense": "destination", "eval": "frame", "interpolation": "linear"}
	for i, corner := range [][2]float64{{0, 0}, {cw, 0}, {0, ch}, {cw, ch}} {
		dx, dy := num6(corner[0]-px), num6(corner[1]-py)
		args["x"+strconv.Itoa(i)] = fmt.Sprintf("%s+%s*%s*cos(%s)-%s*%s*sin(%s)", cx, sx, dx, r, sy, dy, r)
		args["y"+strconv.Itoa(i)] = fmt.Sprintf("%s+%s*%s*sin(%s)+%s*%s*cos(%s)", cy, sx, dx, r, sy, dy, r)
	}
	return args
}

// textClipPanel draws a text clip's glass panel while the clip shows; its text is in the run's ASS.
func textClipPanel(base *ffmpeg.Stream, clip ClipPlan) *ffmpeg.Stream {
	enable := clipEnable(clip)
	for _, fx := range clip.Layer.Effects {
		if fx.Type != "glass_panel" {
			continue
		}
		color := fx.Color
		if color == "" {
			color = "#FFFFFF"
		}
		layer := clip.Layer
		x, y := layer.X-int(fx.PaddingX), layer.Y-int(fx.PaddingY)
		w, h := layer.Width+2*int(fx.PaddingX), layer.Height+2*int(fx.PaddingY)
		if fx.Blur > 0 {
			split := base.Split()
			blurred := split.Get("1").Filter("gblur", nil, ffmpeg.KwArgs{"sigma": fx.Blur})
			mask := fmt.Sprintf("between(X,%d,%d)*between(Y,%d,%d)", x, x+w-1, y, y+h-1)
			base = split.Get("0").Overlay(maskAlpha(blurred, mask), eofActionEndAll, ffmpeg.KwArgs{"x": 0, "y": 0, "enable": enable})
		}
		base = base.Filter("drawbox", nil, ffmpeg.KwArgs{"x": x, "y": y, "w": w, "h": h, "color": withOpacity(color, fx.Opacity), "t": "fill", "enable": enable})
	}
	return base
}

func clipWindow(clip ClipPlan) string {
	return fmt.Sprintf("between(t,%s,%s)", ffTime(clip.Start), ffTime(clip.Start+clip.Duration))
}

// applyEffectClip runs an effect clip's stacked adjustments over everything
// composited so far, limited to the clip's window and blended by its mix.
func applyEffectClip(base *ffmpeg.Stream, plan *Plan, clip ClipPlan) *ffmpeg.Stream {
	enable := clipEnable(clip)
	anim := clip.Layer.Animation
	mixed := anim.Has("opacity") || clip.Mix < 1

	original, effected := base, base
	if mixed {
		split := base.Filter("format", ffmpeg.Args{"yuv420p"}).Split()
		original, effected = split.Get("0"), split.Get("1")
	}

	fps := plan.Video.FPS
	w, h := float64(plan.Video.Width), float64(plan.Video.Height)
	if anim.Has(cameraProps...) {
		t := fmt.Sprintf("(in/%d-%s)", fps, ffTime(clip.Start))
		args := transformCorners(anim, t, w, h, w/2, h/2)
		args["enable"] = enable
		effected = effected.Filter("perspective", nil, args)
	}
	for _, a := range clip.Adjustments {
		effected = adjust(effected, a, enable, fps, w, h)
	}

	if !mixed {
		return effected
	}
	m := fmt.Sprintf("%s*%s", num6(clip.Mix), anim.Expr("opacity", fmt.Sprintf("(T-%s)", ffTime(clip.Start)), 1))
	return ffmpeg.Filter([]*ffmpeg.Stream{original, effected.Filter("format", ffmpeg.Args{"yuv420p"})}, "blend", nil, ffmpeg.KwArgs{
		"all_expr": fmt.Sprintf("A+(B-A)*clip(%s,0,1)", m),
		"enable":   enable,
	})
}

func adjust(s *ffmpeg.Stream, a AdjustmentPlan, enable string, fps int, w, h float64) *ffmpeg.Stream {
	switch a.Type {
	case "blur":
		return s.Filter("gblur", nil, ffmpeg.KwArgs{"sigma": a.Radius, "enable": enable})
	case "color":
		s = s.Filter("eq", nil, ffmpeg.KwArgs{"brightness": a.Brightness, "contrast": a.Contrast, "saturation": a.Saturation, "gamma": a.Gamma, "enable": enable})
		if a.Hue != 0 {
			s = s.Filter("hue", nil, ffmpeg.KwArgs{"h": a.Hue, "enable": enable})
		}
		return s
	case "color_filter":
		v := a.Intensity
		switch a.Preset {
		case "grayscale":
			return s.Filter("hue", nil, ffmpeg.KwArgs{"s": 1 - v, "enable": enable})
		case "sepia":
			return s.Filter("colorchannelmixer", nil, ffmpeg.KwArgs{"rr": 1 - .607*v, "rg": .769 * v, "rb": .189 * v, "gr": .349 * v, "gg": 1 - .314*v, "gb": .168 * v, "br": .272 * v, "bg": .534 * v, "bb": 1 - .869*v, "enable": enable})
		case "warm":
			return s.Filter("colorbalance", nil, ffmpeg.KwArgs{"rs": .15 * v, "bs": -.15 * v, "enable": enable})
		case "cool":
			return s.Filter("colorbalance", nil, ffmpeg.KwArgs{"rs": -.15 * v, "bs": .15 * v, "enable": enable})
		default:
			return s.Filter("eq", nil, ffmpeg.KwArgs{"contrast": 1 + v*.6, "enable": enable})
		}
	case "vignette":
		// Opacity 0 leaves the frame untouched and 1 is a heavy vignette (lens angle 0.9 rad).
		return s.Filter("vignette", nil, ffmpeg.KwArgs{"angle": num6(math.Max(a.Opacity*0.9, 0.001)), "enable": enable})
	case "glow":
		split := s.Split()
		halo := split.Get("1").Filter("gblur", nil, ffmpeg.KwArgs{"sigma": a.Radius, "enable": enable})
		return ffmpeg.Filter([]*ffmpeg.Stream{split.Get("0"), halo}, "blend", nil, ffmpeg.KwArgs{"all_mode": "screen", "all_opacity": a.Strength, "enable": enable})
	case "rgb_split":
		shift := int(math.Round(a.Amount))
		return s.Filter("rgbashift", nil, ffmpeg.KwArgs{"rh": -shift, "bh": shift, "enable": enable})
	case "grain":
		return s.Filter("noise", nil, ffmpeg.KwArgs{"alls": int(math.Round(a.Amount)), "allf": "t+u", "enable": enable})
	case "tint":
		return s.Filter("drawbox", nil, ffmpeg.KwArgs{"x": 0, "y": 0, "w": "iw", "h": "ih", "color": withOpacity(a.Color, a.Opacity), "t": "fill", "enable": enable})
	case "shake":
		// Translate the frame on two incommensurate sines, zoomed just enough to hide the edges.
		t := fmt.Sprintf("(in/%d)", fps)
		zoom := 1 + (2*a.Amount+2)/math.Min(w, h)
		dx := fmt.Sprintf("%s*sin(2*PI*%s*%s)", num6(a.Amount), num6(a.Frequency), t)
		dy := fmt.Sprintf("%s*cos(2*PI*%s*%s*1.37)", num6(a.Amount), num6(a.Frequency), t)
		args := ffmpeg.KwArgs{"sense": "destination", "eval": "frame", "enable": enable}
		for i, c := range [][2]float64{{0, 0}, {w, 0}, {0, h}, {w, h}} {
			args["x"+strconv.Itoa(i)] = fmt.Sprintf("%s+%s", num6(w/2+(c[0]-w/2)*zoom), dx)
			args["y"+strconv.Itoa(i)] = fmt.Sprintf("%s+%s", num6(h/2+(c[1]-h/2)*zoom), dy)
		}
		return s.Filter("perspective", nil, args)
	}
	return s
}
