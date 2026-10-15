package render

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"strings"
	"time"
)

// Advanced mode is opt-in: documents without an `advanced` block compile
// exactly as before. Templates may define element animations, reusable
// keyframe and effect presets, and tracks; videos may add their own presets
// and tracks and disable template tracks. Template tracks stack below video
// tracks, each list in document order.
//
// The clip builder here is shared with custom layouts, whose elements are the
// same clips rendered as the scene's own layout.

var (
	visualProps = map[string]bool{"x": true, "y": true, "scale": true, "scale_x": true, "scale_y": true, "rotation": true, "opacity": true}
	effectProps = map[string]bool{"x": true, "y": true, "scale": true, "scale_x": true, "scale_y": true, "rotation": true, "opacity": true}
	audioProps  = map[string]bool{"volume": true}
	// $(NAME), ${NAME}, either with |fallback; $$ is a literal $.
	bindingRe = regexp.MustCompile(`\$\$|\$\(([A-Za-z0-9_.-]+)(?:\|([^)]*))?\)|\$\{([A-Za-z0-9_.-]+)(?:\|([^}]*))?\}`)
)

type advancedContext struct {
	plan       *Plan
	base       string
	template   obj
	video      obj
	keyframes  obj // named keyframe presets
	effects    obj // named effect stacks
	typography obj
	theme      obj
	media      obj
	audio      obj
}

func newAdvancedContext(p *Plan, v, t obj, base string) *advancedContext {
	tadv, vadv := object(t["advanced"]), object(v["advanced"])
	defaults := object(t["defaults"])
	return &advancedContext{
		plan: p, base: base, template: t, video: v,
		keyframes:  merge(object(tadv["keyframes"]), object(vadv["keyframes"])),
		effects:    merge(object(tadv["effects"]), object(vadv["effects"])),
		typography: object(defaults["typography"]),
		theme:      object(defaults["theme"]),
		media:      object(defaults["media"]),
		audio:      object(defaults["audio"]),
	}
}

func applyAdvanced(ctx *advancedContext) error {
	p, t, v := ctx.plan, ctx.template, ctx.video
	tadv, vadv := object(t["advanced"]), object(v["advanced"])
	if len(tadv) == 0 && len(vadv) == 0 {
		return nil
	}

	if master := merge(object(tadv["master"]), object(vadv["master"])); len(master) > 0 {
		p.Master = &MasterPlan{Loudness: num(choose(master["loudness"], jsonNumber(-14))), TruePeak: num(choose(master["true_peak"], jsonNumber(-1)))}
	}

	if err := ctx.animateElements(object(tadv["animate"])); err != nil {
		return err
	}

	disabled := map[string]bool{}
	for _, id := range list(vadv["disable_tracks"]) {
		disabled[str(id)] = true
	}
	seen := map[string]bool{}
	var tracks []obj
	for _, source := range []obj{tadv, vadv} {
		for _, raw := range list(source["tracks"]) {
			track := object(raw)
			id := str(track["id"])
			if seen[id] {
				return fmt.Errorf("duplicate track id %q", id)
			}
			seen[id] = true
			if !disabled[id] && track["visible"] != false {
				tracks = append(tracks, track)
			}
		}
	}
	for id := range disabled {
		if !seen[id] {
			return fmt.Errorf("disable_tracks: unknown track %q", id)
		}
	}

	for i, track := range tracks {
		if err := ctx.addTrack(track, i); err != nil {
			return fmt.Errorf("track %q: %w", track["id"], err)
		}
	}
	return nil
}

func merge(a, b obj) obj {
	out := obj{}
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
}

// deepMerge overlays b on a, merging nested objects and replacing other values.
func deepMerge(a, b obj) obj {
	out := merge(a, nil)
	for k, v := range b {
		if sub, ok := v.(map[string]any); ok {
			if base, ok := out[k].(map[string]any); ok {
				out[k] = deepMerge(base, sub)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

// animateElements attaches template keyframes to standard layout elements.
func (c *advancedContext) animateElements(animate obj) error {
	for si := range c.plan.Scenes {
		scene := &c.plan.Scenes[si]
		specs := object(animate[scene.Role])
		if len(specs) == 0 {
			continue
		}
		for li := range scene.Layers {
			layer := &scene.Layers[li]
			spec := object(specs[layer.Element])
			if layer.Element == "" || len(spec) == 0 {
				continue
			}
			frames, err := c.keyframeList(spec["keyframes"])
			if err != nil {
				return fmt.Errorf("animate.%s.%s: %w", scene.Role, layer.Element, err)
			}
			anim, err := parseKeyframes(frames, scene.Duration, visualProps)
			if err != nil {
				return fmt.Errorf("animate.%s.%s: %w", scene.Role, layer.Element, err)
			}
			if anim == nil {
				continue
			}
			if layer.Kind == "text" && hasEffect(layer.Effects, "glass_panel") {
				return fmt.Errorf("animate.%s.%s: animated text cannot use glass_panel", scene.Role, layer.Element)
			}
			layer.Animation = anim
		}
	}
	return nil
}

// keyframeList resolves a preset name or an inline list whose items may
// splice presets with {use: name}.
func (c *advancedContext) keyframeList(v any) ([]any, error) {
	return c.resolvePresets(v, c.keyframes, "keyframe")
}

func (c *advancedContext) resolvePresets(v any, presets obj, kind string) ([]any, error) {
	if name, ok := v.(string); ok {
		preset, ok := presets[name]
		if !ok {
			return nil, fmt.Errorf("unknown %s preset %q", kind, name)
		}
		return c.resolvePresets(preset, obj{}, kind)
	}
	var out []any
	for _, raw := range list(v) {
		item := object(raw)
		if use, ok := item["use"]; ok {
			spliced, err := c.resolvePresets(use, presets, kind)
			if err != nil {
				return nil, err
			}
			out = append(out, spliced...)
			continue
		}
		out = append(out, raw)
	}
	return out, nil
}

// owner is the timeline a track is attached to: one scene or the whole video.
type owner struct {
	scene    *ScenePlan // nil for the video
	index    int
	length   time.Duration
	bindings map[string]string
}

// owners resolves a track's attach value: the video, a role, a layout name,
// or a scene id.
func (c *advancedContext) owners(attach string) ([]owner, error) {
	common := map[string]string{"duration": num6(c.plan.Duration.Seconds())}
	for k, v := range object(c.video["metadata"]) {
		if s, ok := v.(string); ok {
			common["metadata."+k] = s
		}
	}
	if attach == "video" {
		return []owner{{index: -1, length: c.plan.Duration, bindings: common}}, nil
	}

	role := attach == "intro" || attach == "slide" || attach == "outro"
	var result []owner
	for i := range c.plan.Scenes {
		scene := &c.plan.Scenes[i]
		if attach != scene.Role && attach != scene.ID && (role || attach != scene.Layout) {
			continue
		}
		bindings := merge2(common, scene.bindings)
		result = append(result, owner{scene: scene, index: i, length: scene.Duration, bindings: bindings})
	}
	if len(result) == 0 && !role && !c.isLayout(attach) {
		return nil, fmt.Errorf("attach %q matches no scene or layout", attach)
	}
	return result, nil
}

func (c *advancedContext) isLayout(name string) bool {
	_, ok := object(c.template["layout"])[name]
	return ok
}

func merge2(a, b map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
}

// missingBinding reports a $(NAME) without a value or fallback.
type missingBinding struct{ name string }

func (e missingBinding) Error() string { return fmt.Sprintf("unknown variable $(%s)", e.name) }

// bind substitutes $(NAME), ${NAME}, and their |fallback forms.
func bind(value string, bindings map[string]string) (string, error) {
	var missing error
	out := bindingRe.ReplaceAllStringFunc(value, func(m string) string {
		if m == "$$" {
			return "$"
		}
		g := bindingRe.FindStringSubmatch(m)
		name, fallback, hasFallback := g[1], g[2], strings.Contains(m, "|")
		if name == "" {
			name, fallback = g[3], g[4]
		}
		if v, ok := bindings[name]; ok {
			return v
		}
		if !hasFallback && missing == nil {
			missing = missingBinding{name}
		}
		return fallback
	})
	return out, missing
}

func (c *advancedContext) addTrack(track obj, trackIndex int) error {
	owners, err := c.owners(str(choose(track["attach"], "video")))
	if err != nil {
		return err
	}
	targeted := map[int]bool{}
	for _, o := range owners {
		// The offset delays the whole track; its clips are timed within the
		// window from the offset to the owner's end.
		shift, err := resolvePoint(choose(track["offset"], "0s"), o.length)
		if err != nil || shift >= o.length {
			return fmt.Errorf("offset %v must start before the end of its owner (%s)", track["offset"], o.length)
		}
		window := o
		window.length -= shift

		plan := TrackPlan{ID: str(track["id"]), ASSPrefix: fmt.Sprintf("video-track-%02d", trackIndex)}
		if o.scene != nil {
			plan.ASSPrefix = fmt.Sprintf("scene-%04d-track-%02d", o.index, trackIndex)
		}
		for ci, raw := range list(track["clips"]) {
			clip := object(raw)
			if str(clip["type"]) == "element" {
				hit, err := c.targetElement(clip, window, shift)
				if err != nil {
					return fmt.Errorf("clip %d (element %q): %w", ci, clip["target"], err)
				}
				targeted[ci] = targeted[ci] || hit
				continue
			}
			if err := c.addClip(&plan, clip, window, shift); err != nil {
				return fmt.Errorf("clip %d (%s): %w", ci, clip["type"], err)
			}
		}
		if len(plan.Clips) == 0 {
			continue
		}
		if o.scene == nil {
			c.plan.Tracks = append(c.plan.Tracks, plan)
		} else {
			o.scene.Tracks = append(o.scene.Tracks, plan)
		}
	}
	for ci, raw := range list(track["clips"]) {
		if clip := object(raw); str(clip["type"]) == "element" && !targeted[ci] && len(owners) > 0 {
			return fmt.Errorf("clip %d: target %q matches no element in the attached scenes", ci, clip["target"])
		}
	}
	return nil
}

// clipTiming resolves a clip's start and length within its window o, which
// begins shift after the owner's start; the start returned is owner time.
func clipTiming(clip obj, o owner, shift time.Duration) (time.Duration, time.Duration, error) {
	start, err := resolvePoint(choose(clip["start"], "0s"), o.length)
	if err != nil {
		return 0, 0, fmt.Errorf("start: %w", err)
	}
	length := o.length - start
	if clip["duration"] != nil {
		if length, err = duration(clip["duration"]); err != nil {
			return 0, 0, err
		}
	}
	if length <= 0 || start+length > o.length {
		return 0, 0, fmt.Errorf("clip %s–%s exceeds its owner (%s)", shift+start, shift+start+length, o.length+shift)
	}
	return start + shift, length, nil
}

// addClip compiles one clip onto a track (or, for audio, into its owner's mix).
func (c *advancedContext) addClip(track *TrackPlan, clip obj, o owner, shift time.Duration) error {
	start, length, err := clipTiming(clip, o, shift)
	if err != nil {
		return err
	}
	clips, err := c.buildClip(clip, o, start, length)
	var missing missingBinding
	if errors.As(err, &missing) && clip["optional"] == true {
		return nil
	}
	if err != nil {
		return err
	}
	track.Clips = append(track.Clips, clips...)
	return nil
}

// buildClip compiles a clip definition placed at start for length on owner o.
// Audio clips go straight into the owner's mix and return no visual clips.
func (c *advancedContext) buildClip(clip obj, o owner, start, length time.Duration) ([]ClipPlan, error) {
	kind := str(clip["type"])
	allowed := visualProps
	switch kind {
	case "audio":
		allowed = audioProps
	case "effect":
		allowed = effectProps
	}
	frames, err := c.keyframeList(clip["keyframes"])
	if err != nil {
		return nil, err
	}
	anim, err := parseKeyframes(frames, length, allowed)
	if err != nil {
		return nil, err
	}

	video := c.plan.Video
	plan := ClipPlan{ID: str(clip["id"]), Kind: kind, Start: start, Duration: length, def: clip, bindings: o.bindings}
	switch kind {
	case "image":
		value, err := bind(str(clip["path"]), o.bindings)
		if err != nil {
			return nil, err
		}
		source, err := path(c.base, value)
		if err != nil {
			return nil, err
		}
		layer, err := mediaLayer(source, clip, c.media, video)
		if err != nil {
			return nil, err
		}
		layer.Opacity = num(choose(clip["opacity"], jsonNumber(1)))
		layer.Duration = length
		layer.Animation = anim
		if err := validateEffects(layer, length); err != nil {
			return nil, err
		}
		plan.Layer = layer
	case "rect", "circle":
		w, h := integer(clip["width"]), integer(clip["height"])
		if w < 1 || h < 1 {
			return nil, fmt.Errorf("width and height must be positive")
		}
		color, err := bind(str(clip["color"]), o.bindings)
		if err != nil {
			return nil, err
		}
		if !v2ASSColorSupported(color) {
			return nil, fmt.Errorf("color %q must be #RRGGBB", color)
		}
		x, y := bounds(clip, video, w, h)
		shape := "rectangle"
		if kind == "circle" {
			shape = "circle"
		} else if clip["corner_radius"] != nil {
			shape = "rounded-rectangle"
		}
		plan.Layer = LayerPlan{Kind: kind, X: x, Y: y, Width: w, Height: h, Color: color, Shape: shape,
			CornerRadius: integer(clip["corner_radius"]), Opacity: num(choose(clip["opacity"], jsonNumber(1))), Duration: length, Animation: anim}
	case "text":
		value, err := bind(str(clip["text"]), o.bindings)
		if err != nil {
			return nil, err
		}
		layer, err := textLayer(value, "text", clip, c.typography, c.theme, video)
		if err != nil {
			return nil, err
		}
		layer.Align = str(object(clip["style"])["align"])
		layer.Duration = length
		layer.Animation = anim
		if anim != nil && hasEffect(layer.Effects, "glass_panel") {
			return nil, fmt.Errorf("animated text cannot use glass_panel")
		}
		if err := validateEffects(layer, length); err != nil {
			return nil, err
		}
		plan.Layer = layer
	case "progress":
		if o.scene == nil || o.scene.Role != "slide" {
			return nil, fmt.Errorf("progress needs a slide")
		}
		if anim != nil {
			return nil, fmt.Errorf("progress cannot be animated")
		}
		var bars ScenePlan
		if err := addProgress(&bars, clip, o.scene.index+1, o.scene.total, c.theme, video); err != nil {
			return nil, err
		}
		var out []ClipPlan
		for _, bar := range bars.Layers {
			box := plan
			box.Kind, box.Layer = "box", bar
			out = append(out, box)
		}
		return out, nil
	case "audio":
		value, err := bind(str(clip["path"]), o.bindings)
		if err != nil {
			return nil, err
		}
		a, err := audioLayer(merge(clip, obj{"path": value}), c.audio, c.base, start, length)
		if err != nil {
			return nil, err
		}
		a.SoundEffect = clip["ducks"] != true
		a.VolumeAnimation = anim
		if o.scene == nil {
			c.plan.Audio = append(c.plan.Audio, a)
		} else {
			o.scene.Audio = append(o.scene.Audio, a)
		}
		return nil, nil
	case "effect":
		effect, err := c.effectClip(clip, length)
		if err != nil {
			return nil, err
		}
		effect.ID, effect.Start, effect.def, effect.bindings = plan.ID, start, clip, o.bindings
		plan = effect
	case "element":
		return nil, fmt.Errorf("element clips target layout elements and belong in advanced tracks")
	default:
		return nil, fmt.Errorf("unsupported clip type %q", kind)
	}
	return []ClipPlan{plan}, nil
}

// targetElement applies an element clip to the layout element it targets in
// o's scene: keyframes animate the element, and set modifies it during the
// clip by showing a recompiled variant in its place.
func (c *advancedContext) targetElement(clip obj, o owner, shift time.Duration) (bool, error) {
	if o.scene == nil {
		return false, fmt.Errorf("element clips need a scene; attach the track to a role, layout, or scene")
	}
	start, length, err := clipTiming(clip, o, shift)
	if err != nil {
		return false, err
	}
	frames, err := c.keyframeList(clip["keyframes"])
	if err != nil {
		return false, err
	}
	anim, err := parseKeyframes(frames, length, visualProps)
	if err != nil {
		return false, err
	}
	set := object(clip["set"])
	if anim == nil && len(set) == 0 {
		return false, fmt.Errorf("element clips need keyframes or set")
	}
	hide := set["visible"] == false
	if v, ok := set["visible"]; ok && v != false && v != true {
		return false, fmt.Errorf("set.visible must be true or false")
	}
	set = merge(set, nil)
	delete(set, "visible")
	target := str(clip["target"])
	window := Window{start, start + length}
	scene := o.scene
	hit := false

	// Classic layout elements: their timeline starts with the scene.
	for i := 0; i < len(scene.Layers); i++ {
		layer := &scene.Layers[i]
		if layer.Element != target {
			continue
		}
		hit = true
		if len(set) == 0 && !hide {
			layer.Animation = overrideAnimation(layer.Animation, anim, start)
			if err := checkAnimatedPanel(*layer); err != nil {
				return true, err
			}
			continue
		}
		shown, overlaps := variantWindow(*layer, window)
		if !overlaps {
			continue
		}
		layer.HideDuring = append(layer.HideDuring, shown)
		if hide {
			continue
		}
		variant, err := c.classicVariant(*scene, *layer, set)
		if err != nil {
			return true, err
		}
		variant.ShowOnly = &shown
		variant.HideDuring = nil
		variant.Animation = overrideAnimation(layer.Animation, anim, start)
		if err := checkAnimatedPanel(variant); err != nil {
			return true, err
		}
		scene.Layers = append(scene.Layers[:i+1], append([]LayerPlan{variant}, scene.Layers[i+1:]...)...)
		i++
	}

	// Custom layout elements: their timeline starts with the element.
	for ti := range scene.Tracks {
		track := &scene.Tracks[ti]
		if track.ID != "layout" {
			continue
		}
		for i := 0; i < len(track.Clips); i++ {
			element := &track.Clips[i]
			if element.ID != target {
				continue
			}
			hit = true
			if element.Kind == "box" {
				return true, fmt.Errorf("progress elements cannot be animated or modified")
			}
			if len(set) == 0 && !hide {
				element.Layer.Animation = overrideAnimation(element.Layer.Animation, anim, start-element.Start)
				if err := checkAnimatedPanel(element.Layer); err != nil {
					return true, err
				}
				continue
			}
			shown, overlaps := variantWindow(element.Layer, window)
			if !overlaps {
				continue
			}
			element.Layer.HideDuring = append(element.Layer.HideDuring, shown)
			if hide {
				continue
			}
			modified := deepMerge(element.def, set)
			if err := validateDefinition(modified, "element"); err != nil {
				return true, err
			}
			owner := owner{scene: scene, index: o.index, length: scene.Duration, bindings: element.bindings}
			variants, err := c.buildClip(modified, owner, element.Start, element.Duration)
			if err != nil {
				return true, err
			}
			for v := range variants {
				variants[v].Layer.ShowOnly = &shown
				variants[v].Layer.HideDuring = nil
				variants[v].Layer.Animation = overrideAnimation(element.Layer.Animation, anim, start-element.Start)
				if err := checkAnimatedPanel(variants[v].Layer); err != nil {
					return true, err
				}
			}
			track.Clips = append(track.Clips[:i+1], append(variants, track.Clips[i+1:]...)...)
			i += len(variants)
		}
	}
	return hit, nil
}

// variantWindow is when a set-clip variant of layer shows: the clip's window,
// narrowed to the layer's own window when the layer is itself a variant from an
// earlier clip. It reports false when they don't overlap; the layer is then left
// alone, so clips with separate windows don't multiply each other's variants.
func variantWindow(layer LayerPlan, window Window) (Window, bool) {
	if layer.ShowOnly == nil {
		return window, true
	}
	shown := Window{max(window.Start, layer.ShowOnly.Start), min(window.End, layer.ShowOnly.End)}
	return shown, shown.Start < shown.End
}

func checkAnimatedPanel(layer LayerPlan) error {
	if layer.Kind == "text" && layer.Animation != nil && hasEffect(layer.Effects, "glass_panel") {
		return fmt.Errorf("animated text cannot use glass_panel")
	}
	return nil
}

// classicVariant recompiles a classic layout element with set applied: text
// and path replace its content, other keys merge into its layout definition.
func (c *advancedContext) classicVariant(scene ScenePlan, layer LayerPlan, set obj) (LayerPlan, error) {
	content := layer.source
	key := "text"
	if layer.Kind != "text" {
		key = "path"
	}
	if v, ok := set[key]; ok {
		bound, err := bind(str(v), scene.bindings)
		if err != nil {
			return LayerPlan{}, err
		}
		content = bound
		if key == "path" {
			if content, err = path(c.base, bound); err != nil {
				return LayerPlan{}, err
			}
		}
	}
	el := deepMerge(layer.def, set)
	delete(el, key)
	if layer.Kind != "text" {
		delete(el, "opacity")
	}
	if layer.def != nil {
		def := "media_element"
		if layer.Kind == "text" {
			def = "text_element"
		}
		if err := validateDefinition(el, def); err != nil {
			return LayerPlan{}, err
		}
	}

	var variant LayerPlan
	var err error
	switch {
	case layer.Kind == "text":
		variant, err = textLayer(content, layer.Element, el, c.typography, c.theme, c.plan.Video)
		variant.Align = str(object(el["style"])["align"])
	case layer.def == nil: // intro/outro full-frame background image
		variant = layer
		variant.Path = content
	default:
		variant, err = mediaLayer(content, el, c.media, c.plan.Video)
		variant.AltText = layer.AltText
	}
	if err != nil {
		return LayerPlan{}, err
	}
	variant.Element, variant.def, variant.source = layer.Element, el, content
	if variant.Kind != "text" {
		variant.Opacity = num(choose(set["opacity"], jsonNumber(layer.Opacity)))
	}
	if variant.Opacity < 0 || variant.Opacity > 1 || math.IsNaN(variant.Opacity) {
		return LayerPlan{}, fmt.Errorf("opacity must be between 0 and 1")
	}
	return variant, validateEffects(variant, scene.Duration)
}

// overrideAnimation returns base with the properties animated by add replaced
// by add's keyframes, shifted by offset into base's timeline.
func overrideAnimation(base, add *Animation, offset time.Duration) *Animation {
	if add == nil {
		return base
	}
	out := &Animation{Props: map[string][]Keyframe{}}
	if base != nil {
		maps.Copy(out.Props, base.Props)
	}
	for name, frames := range add.Props {
		shifted := make([]Keyframe, len(frames))
		for i, f := range frames {
			f.At += offset
			shifted[i] = f
		}
		out.Props[name] = shifted
	}
	return out
}

// effectClip builds an effect layer of the given length from effects (a stack
// or preset name), keyframes (mix opacity and camera), and opacity.
func (c *advancedContext) effectClip(spec obj, length time.Duration) (ClipPlan, error) {
	frames, err := c.keyframeList(spec["keyframes"])
	if err != nil {
		return ClipPlan{}, err
	}
	anim, err := parseKeyframes(frames, length, effectProps)
	if err != nil {
		return ClipPlan{}, err
	}
	stack, err := c.resolvePresets(spec["effects"], c.effects, "effect")
	if err != nil {
		return ClipPlan{}, err
	}
	if len(stack) == 0 && !anim.Has(cameraProps...) {
		return ClipPlan{}, fmt.Errorf("effect layer needs effects or camera keyframes")
	}
	clip := ClipPlan{Kind: "effect", Duration: length, Mix: num(choose(spec["opacity"], jsonNumber(1))),
		Layer: LayerPlan{Kind: "effect", Width: c.plan.Video.Width, Height: c.plan.Video.Height, Animation: anim}}
	for _, raw := range stack {
		adj, err := adjustment(object(raw))
		if err != nil {
			return ClipPlan{}, err
		}
		clip.Adjustments = append(clip.Adjustments, adj)
	}
	return clip, nil
}

var adjustmentTypes = map[string]bool{
	"blur": true, "color": true, "color_filter": true, "vignette": true, "glow": true,
	"rgb_split": true, "grain": true, "tint": true, "shake": true,
}

func adjustment(v obj) (AdjustmentPlan, error) {
	typ := str(v["type"])
	if !adjustmentTypes[typ] {
		return AdjustmentPlan{}, fmt.Errorf("unsupported effect %q", typ)
	}
	a := AdjustmentPlan{
		Type: typ, Radius: num(choose(v["radius"], jsonNumber(8))), Strength: num(choose(v["strength"], jsonNumber(0.6))),
		Amount: num(choose(v["amount"], jsonNumber(6))), Brightness: num(v["brightness"]), Contrast: num(choose(v["contrast"], jsonNumber(1))),
		Saturation: num(choose(v["saturation"], jsonNumber(1))), Gamma: num(choose(v["gamma"], jsonNumber(1))), Hue: num(v["hue"]),
		Frequency: num(choose(v["frequency"], jsonNumber(12))), Color: str(choose(v["color"], "#FFFFFF")),
		Opacity: num(choose(v["opacity"], jsonNumber(0.5))), Preset: str(v["preset"]), Intensity: num(choose(v["intensity"], jsonNumber(1))),
	}
	if typ == "color_filter" {
		switch a.Preset {
		case "grayscale", "sepia", "warm", "cool", "high-contrast":
		default:
			return a, fmt.Errorf("unsupported color preset %q", a.Preset)
		}
	}
	if strings.HasPrefix(a.Color, "#") && len(a.Color) != 7 {
		return a, fmt.Errorf("effect color must be #RRGGBB")
	}
	return a, nil
}
