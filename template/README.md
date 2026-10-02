# Slide templates

A project remains a normal project document. Templates expand `slides:` into
the standard `scenes:` representation and may provide fallback `video`,
`defaults`, and `music` settings.

Programmatic callers load all projects through `project.Load`:

```go
value, err := project.Load("presentation.yaml", project.LoadOptions{
    TemplatePaths: []string{"brand-template.yaml"},
})
```

## Use templates

Supply one or more template files with repeatable `-t` flags:

```sh
yaml2video -t appdemo-template.yaml -t brand-template.yaml presentation.yaml
```

Template files are not declared inside the project YAML. The output filename is
also controlled by the normal `-o` CLI option.

## Project with slides

Aside from `slides:` replacing `scenes:`, this is a standard project document:

```yaml
version: 1
video:
  width: 1920
  height: 1080
  fps: 30
  background: "#101010"

defaults:
  text:
    font: Arial
    size: 48
    color: white

music:
  path: music.mp3
  volume: 0.5
  fade_out: 1

slides:
  - type: intro
    image: intro.jpg
    text: Stadt Stuttgart

  - type: screenshot
    image: stuttgart-map.png
    textline1: Unfälle in Stuttgart?
    textline2: kein Problem mehr!
    textline3: mit der neuen Unfall-Atlas-App
    duration: 5s
```

Slide durations accept a numeric seconds value (`5`) or a Go-style duration
string (`5s`, `1.5s`).

## Template file

A template file contains one or more `slide_templates`. The body uses normal
scene fields (`background`, `layers`, `subtitles`, and `transition`) but omits
`id` and can omit `duration`.

```yaml
slide_templates:
  - name: intro
    required: [image, text]
    duration: 5s
    background: "#101010"
    layers:
      - type: image
        path: "{{image}}"
        fit: cover
        transform: center
      - type: text
        text:
          - "{{text}}"
        transform: bottom-center
```

Placeholders accept `{{image}}`, `{{ image }}`, and `{{.image}}`. They are
replaced from fields on the corresponding slide. A slide may provide `id` to
override its generated ID (`<type>-<position>`) and may override the template
duration. Required fields are checked before expansion.

A template file may also define top-level `video:`, `defaults:`, and `music:`
sections. They are used only when the project omits the matching section; the
project's section always wins as a whole.

## Text backgrounds

Text and subtitle defaults can give every text item a background. `padding` is
either one value for both axes or `[x, y]` for separate horizontal and vertical
padding:

```yaml
defaults:
  text:
    background:
      color: black
      opacity: 0.6
      padding: [12, 6]
  subtitle:
    background:
      color: black
      padding: 8
```

A `type: text` layer can set one singular background box behind its complete
rendered text, avoiding per-item repetition. A long-form text item can still
add its own background override:

```yaml
- type: text
  background:
    color: black
    opacity: 0.6
    padding: [12, 6]
  text:
    - "Uses the one layer background"
    - content: "Adds a red item background"
      background:
        color: red
        opacity: 0.5
        padding: [4, 2]
```

## Layer dimensions, scaling, and video trim

Every layer accepts optional pixel `width` and `height`. Image and video layers
apply their `fit` mode to those dimensions. A text layer with `width` wraps its
text to that width. When text omits either dimension, its size is calculated
from the rendered text content; text-layer backgrounds use that calculated box
plus their configured padding.

```yaml
- type: image
  path: product.png
  width: 640
  height: 360
  fit: contain
  transform:
    x: 0.5
    y: 0.5
    scale_x: 1.2
    scale_y: 0.8
```

`transform.scale_x` and `scale_y` are actual scale factors applied after pixel
dimensions are resolved, so they can stretch or squash a layer. A non-unit
scale requires the corresponding explicit `width` or `height`; it never falls
back to scaling the canvas. When a custom transform omits either scale, it
defaults to `1`.

Video layers can crop fractional insets from the source before fitting:

```yaml
- type: video
  path: source.mp4
  width: 640
  height: 360
  fit: cover
  trim: [0.1, 0.2] # 10% top/bottom; 20% left/right
```

`trim: 0.1` applies the same fraction to all four sides. A four-item trim is
`[top, bottom, left, right]`. Opposite sides must total less than `1`.

Duplicate slide-template names across `-t` files are an error.
