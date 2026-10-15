# Advanced Mode: Tracks, Keyframes, and Effect Layers

Advanced mode is an optional `advanced` block in a template, a video, or both. Documents without it render exactly as before; nothing in the standard workflow changes. The [advanced example](../examples/advanced/) shows every feature, together with [named and custom layouts](layouts.md), on top of the simple example.

| Feature | Template | Video |
| --- | --- | --- |
| `animate`: keyframes for standard layout elements | ✓ | — |
| `keyframes`: named keyframe presets | ✓ | ✓ (extends/overrides) |
| `effects`: named effect-layer stacks | ✓ | ✓ (extends/overrides) |
| `tracks`: timed clips and effect layers | ✓ (rendered first) | ✓ (rendered above) |
| `disable_tracks`: drop template tracks by id | — | ✓ |
| `transitions`: named custom transitions | ✓ | ✓ (extends/overrides) |
| `scene_transitions`: per-scene in/out overrides | — | ✓ |
| `master`: loudness-normalize the final mix | ✓ | ✓ (overrides) |

## Keyframes

A keyframe list sets values at moments. Each property animates between the keyframes that set it; `ease` shapes the segment that *starts* at that keyframe. Before the first keyframe a property holds its first value, and after the last it holds the last.

```yaml
keyframes:
    - { at: 0s, scale: 0.6, opacity: 0, ease: back-out }
    - { at: 0.5s, scale: 1, opacity: 1 }
    - { at: -0.4s, opacity: 1, ease: ease-in }   # 0.4 s before the end
    - { at: -0s, opacity: 0 }                     # the very end
```

- `at` is seconds from the owner's start (the element's scene or the clip), or a negative string counted back from its end, so the same preset works for scenes of any length.
- Properties: `x`, `y` (pixel offsets from the rest position), `scale`, `scale_x`, `scale_y`, `rotation` (degrees clockwise around the center), `opacity` (0–1), and `volume` (0–1, audio clips only).
- Easings: `linear`, `step` (hold until the next keyframe), `ease-in`, `ease-out`, `ease-in-out`, `back-in`, `back-out` (overshoot), `elastic-out`, `bounce-out`.
- Wherever keyframes are accepted you may write a preset name (`keyframes: pop-in`) or a list whose items splice presets (`- { use: pop-in }`).

Keyframed images and shapes are transformed with FFmpeg's `perspective` and `geq` filters; keyframed text is sampled every 50 ms into ASS `\move`/`\t` segments, so every easing curve is followed closely.

## Animating layout elements (template)

`animate.<role>.<element>` attaches keyframes to standard elements: `intro` (`logo`, `title`, `subtitle`), `slide` (`logo`, `image`, `label`, `text`, `tip`, `index`), and `outro` (`logo`, `title`, `text`, `cta`). Times are relative to each scene.

```yaml
advanced:
    animate:
        slide:
            image: { keyframes: card-float }
            text:
                keyframes:
                    - { at: 0.15s, y: 70, opacity: 0, ease: back-out }
                    - { at: 0.65s, y: 0, opacity: 1 }
```

A `fade` effect on an animated text element is folded into its opacity. Animated text cannot use `glass_panel`, whose panel would stay behind.

## Tracks and clips

A track is a stack layer. Tracks render in order above the content they attach to; template tracks render before (below) video tracks. Within a track, clips stack in list order, text included. Track clips are the same element kinds a [custom layout](layouts.md#custom-elements) uses, plus `element` clips that change the layout itself (see [Element clips](#element-clips)).

```yaml
tracks:
    - id: slide-counter
      attach: slide            # video (default), intro, slide, outro, or a scene id
      clips:
          - type: circle
            width: 120
            height: 120
            color: "#7DE2A8"
            placement: { anchor: top-right, offset_x: -60, offset_y: 60 }
            keyframes: pop-in
          - type: text
            text: "$(index)/$(total)"
            placement: { anchor: top-right, offset_x: -60, offset_y: 98 }
            style: { font_size: 38, weight: bold, color: "#101820", align: center }
            max_width_ratio: 0.111
```

`attach` decides the clip timeline:

- `video` places clips on the final timeline, above all scenes and transitions. Use absolute `start` times.
- `intro`, `slide`, or `outro` repeats the track in every scene with that role, so a template track decorates every slide. Clip times are relative to each scene, and a role with no scenes is skipped.
- A layout name repeats the track in every scene that uses that layout.
- A scene id such as `exercises-nacken` (section id plus slide id) targets one scene.

A track's optional `offset` delays the whole track from its owner's start. A negative `offset` counts back from the owner's end, so `offset: -1s` places the track in the last second of every scene. Clip times are then relative to the track window, which runs from the offset to the owner's end. Shifting a track's offset moves all of its clips together:

```yaml
- id: lower-third
  attach: slide
  offset: 0.8s            # everything below starts 0.8 s into each slide
  clips:
      - { type: rect, width: 600, height: 90, color: "#101820", placement: { anchor: bottom-center, offset_y: -300 } }
      - { type: text, text: "$(text)", start: 0.2s, placement: { anchor: bottom-center, offset_y: -320 } }
```

Every clip takes `start` (default `0s`, negative counts back from the window's end), `duration` (default: to the window's end), and `keyframes`. A clip must fit inside its track window.

| `type` | Fields |
| --- | --- |
| `image` | `path`, `placement`, `max_width_ratio`, `max_height_ratio`, `fit`, `shape`, `opacity`, image `effects` (as in layouts) |
| `rect`, `circle` | `width`, `height` (px), `color` (`#RRGGBB`), `placement`, `corner_radius` (rect), `opacity` |
| `text` | `text`, `placement`, `style`, `max_width_ratio`, `max_lines`, text `effects` (as in layouts; `glass_panel` only without keyframes) |
| `progress` | `placement`, `width_ratio`, `thickness`, `color`, `background_color` (slide scenes only) |
| `audio` | `path`, `volume`, `fade_in`, `fade_out`, `ducks` (true makes it foreground audio that ducks the music) |
| `effect` | `effects` (an effect-layer stack), `opacity` (mix) |
| `element` | `target`, `set`, `keyframes` (see [Element clips](#element-clips)) |

Text, paths, and shape colors take the same `$(NAME)` [variables](layouts.md#variables) as custom layouts (`${NAME}` works too). In scene-attached tracks, a name reads the scene's content item: its `variables`, its fields (`$(title)`, `$(text)`, `$(image)`, …), `$(index)` and `$(total)` for slides, `$(scene)`, `$(role)`, and `$(layout)`. All tracks can use `$(metadata.title)` (and other metadata fields) and `$(duration)`. An unknown name is an error unless it has a `|fallback` or the clip is `optional`, so `path: $(image)` lets a template reuse each slide's own image.

### Element clips

An `element` clip doesn't draw anything itself. It animates or modifies an element of the attached scene's layout: a custom layout element by `id`, or a classic element by name (`title`, `subtitle`, `logo`, `image`, `label`, `text`, `tip`, `index`, `cta`).

```yaml
- id: highlight-text
  attach: slide
  clips:
      - type: element
        target: text
        start: 1.4s
        duration: 0.8s
        set: { style: { color: "#7DE2A8" } }     # green during the clip only
        keyframes:                                # pops while it is green
            - { at: 0s, scale: 1.2, ease: back-out }
            - { at: 0.4s, scale: 1 }
```

- With only `keyframes`, the clip animates the element itself, relative to the clip's start. Its properties replace the element's own keyframes for those properties.
- With `set`, the element shows a modified form during the clip and its original form before and after. `set` accepts any property of the element (`text`, `path`, `color`, `style`, `effects`, `placement`, …, validated against its schema), and `set: { visible: false }` hides it. Keyframes then animate the modified form.

See [targeting layout elements](layouts.md#targeting-layout-elements-advanced-mode) for the rules on matching.

Advanced text uses an outline-style ASS style, so `text_glow` and `text_outline` surround the letters. Standard layout text keeps its original box-style rendering.

## Effect layers

An `effect` clip is an adjustment layer. Its effects run in order over everything composited below it, only during the clip:

| Effect | Parameters (defaults) |
| --- | --- |
| `blur` | `radius` (8) |
| `color` | `brightness` (0), `contrast` (1), `saturation` (1), `gamma` (1), `hue` (degrees, 0) |
| `color_filter` | `preset` (`grayscale`, `sepia`, `warm`, `cool`, `high-contrast`), `intensity` (1) |
| `vignette` | `opacity` (0.5): 0 is none, 1 is heavy |
| `glow` | `radius` (8), `strength` (0.6): a screen-blended bloom |
| `rgb_split` | `amount` (6 px): chromatic aberration |
| `grain` | `amount` (6): animated noise |
| `tint` | `color` (`#FFFFFF`), `opacity` (0.5): a flat color wash; animate opacity for flashes |
| `shake` | `amount` (6 px), `frequency` (12 Hz) |

An effect clip's keyframes animate `opacity`, which blends the processed frame over the original (fades a flash or a glitch in and out), and `x`, `y`, `scale`, and `rotation`, which move a camera over everything below. A clip with only camera keyframes is a pure camera move:

```yaml
- type: effect
  start: 3s
  duration: 0.6s
  keyframes:
      - { at: 0s, scale: 1.12, ease: ease-out }   # punch in on the beat
      - { at: 0.6s, scale: 1 }
```

Named stacks live under `advanced.effects` and are referenced by name (`effects: impact`) or spliced (`- { use: impact }`).

## Transitions

Scenes cut unless a transition is set. Templates choose transitions per section. `transition_out` may be a list that rotates across the section's scenes, and `transition_in` sets the transition into the section's scenes:

```yaml
composition:
    sections:
        - { id: intro, role: intro, source: content.intro, transition_out: { type: flash-cut } }
        - id: exercises
          role: slide
          source: content.slides
          repeat: true
          transition_out:              # slide 1, 2, 3, ... rotate through the list
              - { type: slide-left, duration: 0.4s }
              - { type: smooth-up, duration: 0.4s }
        - { id: outro, role: outro, source: content.outro, transition_in: { type: zoom-blur } }
```

A video can override any scene's transitions by scene id:

```yaml
advanced:
    scene_transitions:
        exercises-nacken:
            out: { type: iris }
            in: { type: fade, duration: 0.3s }
```

### Custom transitions

`advanced.transitions` (template or video; the video's entries win) defines named transitions. Each name works anywhere a built-in name does, and a use may override its `duration`:

```yaml
advanced:
    transitions:
        flash-cut:                 # no base or expr: a hard cut
            duration: 0.4s
            effects: [{ type: tint, color: "#FFFFFF", opacity: 0.85 }]
            keyframes:             # across the boundary, centered on the cut
                - { at: 0s, opacity: 0, ease: ease-in }
                - { at: 0.2s, opacity: 1, ease: ease-out }
                - { at: 0.4s, opacity: 0 }
        zoom-blur:
            duration: 0.3s
            out:                   # end of the outgoing scene only
                effects: [{ type: blur, radius: 10 }]
                keyframes: [{ at: 0s, scale: 1, ease: ease-in }, { at: -0s, scale: 1.35 }]
            in:                    # start of the incoming scene only
                effects: [{ type: blur, radius: 10 }]
                keyframes: [{ at: 0s, scale: 1.35, ease: ease-out }, { at: -0s, scale: 1 }]
        iris:
            duration: 0.6s
            expr: "if(lt(hypot(X-W/2,Y-H/2),P*hypot(W/2,H/2)),B,A)"
```

| Field | Meaning |
| --- | --- |
| `base` | A built-in blend between the scenes. Omit `base` and `expr` for a hard cut (the default). |
| `expr` | A custom blend: an FFmpeg expression evaluated per pixel. `A`/`B` are the outgoing and incoming pixels, `X`, `Y`, `W`, `H` the position and frame size, and `P` the progress from 0 to 1. |
| `duration` | Default length of the blend and of each side. |
| `effects`, `keyframes`, `opacity` | An effect layer across the whole transition (both sides), centered on the boundary. |
| `out` | An effect layer on the end of the outgoing scene only, with its own optional `duration`. |
| `in` | An effect layer on the start of the incoming scene only, with its own optional `duration`. |

**In and out:** a transition given on one side of a boundary covers both sides. When the outgoing scene has an out transition and the incoming scene has an in transition, the out side and the in side come from their own transitions. The blend comes from the out transition, or from the in transition if the out side has none. Shared effects from both are stacked. Side effects render inside their scene, above all of its tracks, so they travel with any blend. Shared effects render on the final timeline.

## Master bus

`amix` scales each input by 1/N, so mixes with many clips come out quiet. `master` loudness-normalizes the final mix:

```yaml
advanced:
    master:
        loudness: -14     # LUFS, default -14
        true_peak: -1.5   # dBTP, default -1
```

## Rendering notes

- Scene-attached tracks are rendered into each scene before transitions, so they move with them. Video tracks are composited in the final pass.
- Each distinct audio file is decoded once and split for every use.
- Run `yaml2video -n` to see every element's animated properties and each track's clips with absolute times.
