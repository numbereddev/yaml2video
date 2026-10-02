# YAML feature reference

This page describes the conventional `scenes` project format. Durations in this format are numeric seconds—for example, `duration: 1.5`. Slide-template durations additionally accept Go duration strings; see [slide templates](templates.md).

## Top-level fields

```yaml
version: 1
video:
  width: 1920
  height: 1080
  fps: 30
  background: "#101010"
defaults:
  text: {}
  subtitle: {}
music:
  path: music.mp3
  volume: 0.5
  fade_out: 1
scenes: []
```

| Field | Behavior |
| --- | --- |
| `version` | Recommended document version marker. The current format is `1`; the renderer does not validate this field. |
| `video` | Required output canvas. `width`, `height`, and `fps` must be positive; `background` must be a supported color. |
| `defaults.text` | Typography and optional background used by text layers. |
| `defaults.subtitle` | Typography and optional background used by subtitles. |
| `music` | Optional looping background-audio track. It is included only when `path` is non-empty; its path may be a local file or an HTTP(S) URL. |
| `scenes` | Required non-empty ordered list of scenes. |

## Colors and opacity

Colors accept named `black`, `white`, `red`, `green`, `blue`, `yellow`, and `transparent`; hexadecimal `#RGB`, `#RGBA`, `#RRGGBB`, or `#RRGGBBAA`; and `rgb(...)` or `rgba(...)` values. Opacity values are numbers from `0` through `1`.

## Scene

```yaml
- id: product
  duration: 5
  background: "#172033"
  layers: []
  subtitles: []
  transition:
    out:
      type: fade
      duration: 0.4
```

`id` must be unique and `duration` must be positive. `background` is optional. `transition.out` defines the overlap to the next scene; its duration must be positive and no longer than this scene. Scene transitions support `fade` and `dissolve`. `transition.in` is not valid for scenes.

## Layers

Layers render in order: later visual layers appear over earlier layers. Every layer accepts optional pixel `width`, `height`, `transform`, and `opacity`. Width and height must be positive when supplied. `opacity` defaults to `1` and must be between `0` and `1`.

### Image and video

```yaml
- type: image
  path: assets/product.png
  width: 960
  height: 540
  fit: contain
  transform: center
  opacity: 0.9

- type: video
  path: assets/demo.mp4
  width: 960
  height: 540
  fit: cover
  source_offset: 2
  duration: 4
  trim: [0.05, 0.1]
  transform: center
```

`path` is required. It may be a local file path or an `http://` or `https://` URL. URLs are passed directly to FFmpeg; the server must expose media in a format supported by the installed FFmpeg build. Quote URLs that include YAML-significant characters such as `#`. `fit` is `cover` by default: it fills the target area and crops excess source content. `contain` preserves the entire source and pads the unused area transparently. Without dimensions, a media layer fills the canvas.

For videos, `source_offset` skips seconds at the beginning of the source and `duration` limits playback. `trim` crops a fraction of the source frame before fitting:

- `trim: 0.1` — 10% from each side.
- `trim: [vertical, horizontal]` — top/bottom then left/right.
- `trim: [top, bottom, left, right]` — four individual sides.

Trim values cannot be negative and opposing sides must total less than `1`. `trim` is valid only on video layers.

### Text

```yaml
- type: text
  size: 54
  width: 900
  background:
    color: black
    opacity: 0.6
    padding: [16, 8]
    radius: 10
  text:
    - "A normal span "
    - content: "with emphasis"
      bold: true
      italic: true
      color: "#FFD400"
  transform: bottom-center
```

`text` must contain at least one item. An item is either a string or an object with `content`, `bold`, `italic`, `underline`, `color`, and an optional per-item `background`. Text uses `defaults.text.font`, `.size`, and `.color`; the layer `size` can override the default. A text-layer `width` enables approximate word wrapping. Without explicit text dimensions, its bounds are calculated from the rendered text.

A text background may be set in `defaults.text`, on the complete text layer, or on a long-form text item. It accepts `color`, optional `opacity` (default `1`), `padding` as one pixel value or `[horizontal, vertical]`, and non-negative `radius`. `radius` is accepted and validated, but current rendering does not round text-background corners.

### Shapes

```yaml
- type: rectangle
  width: 800
  height: 100
  color: "rgba(0,0,0,0.65)"
  opacity: 0.8
  radius: 16
  transform: bottom-center

- type: circle
  width: 160
  height: 160
  color: "#ff0000"
  transform: top-right
```

`rectangle` and `circle` require `color`. Dimensions default to the canvas if omitted. Rectangle rendering uses a filled box, while circle rendering fits a circle inside the supplied bounds. `radius` is accepted for shape layers but does not currently change the rendered shape.

### Audio

```yaml
- type: audio
  path: assets/voiceover.wav
  source_offset: 0.5
  duration: 3
  volume: 0.8
```

`path` is required and may be a local file path or an `http://` or `https://` URL. Audio starts with its containing scene. `volume` defaults to `1` and must not be negative. Audio has no visual output.

## Placement and scaling

Use a preset transform:

```yaml
transform: top-left
```

Supported presets are `center`, `center-left`, `center-right`, `top-center`, `top-left`, `top-right`, `bottom-center`, `bottom-left`, and `bottom-right`.

Or use a custom transform, whose `x` and `y` are normalized canvas coordinates and locate the layer center:

```yaml
transform:
  x: 0.5
  y: 0.25
  scale_x: 1.2
  scale_y: 0.8
```

Custom `x` and `y` must be between `0` and `1`. Scales default to `1` when omitted, must be positive, and require the corresponding explicit `width` or `height` when non-unit. Scaling is applied after pixel dimensions are resolved. The YAML `rotate` field is decoded but does not currently rotate a layer.

## Subtitles

```yaml
subtitles:
  - duration: 1.5
    text: ["The first caption"]
    transition:
      in: { type: fade, duration: 0.15 }
      out: { type: fade, duration: 0.15 }
  - offset: 3
    duration: 1
    text:
      - content: "A highlighted caption"
        color: yellow
```

Subtitles use `defaults.subtitle` for typography and background. They are sequential by default; `offset` places a subtitle at an explicit number of seconds from scene start. Each subtitle must have positive duration, fit entirely within its scene, and may use only `fade` transitions with durations no longer than the subtitle.

## Currently accepted but inactive fields

`defaults.text.align`, `defaults.text.vertical_align`, `defaults.subtitle.transform`, and `defaults.subtitle.max_width` are decoded from YAML but do not currently alter rendering. Use explicit text-layer dimensions and transforms for layout control.
